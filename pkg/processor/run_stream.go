package processor

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/pkg/config"

	"go.opentelemetry.io/otel/attribute"
)

// RunStream processes the source file through the processor pipeline and writes
// the result to a temporary file, returning its path. The caller must remove the
// file when done. If all steps are streamable, data flows through the pipeline
// without full materialization. If any step requires []byte, RunStream falls back
// to reading the intermediate result, running the non-streamable step via Run(),
// and resuming streaming for subsequent steps.
func RunStream(ctx context.Context, steps []config.ProcessorStep, srcPath, tmpDir string) (string, error) {
	started := time.Now()
	ctx, span := observability.Start(ctx, "processor.stream", attribute.Int("processor.steps", len(steps)))
	var opErr error
	defer func() {
		status := "ok"
		if opErr != nil {
			status = "error"
		}
		attrs := []attribute.KeyValue{
			attribute.String("processor.mode", "stream"),
			attribute.String("processor.status", status),
		}
		observability.Count(ctx, "processor.runs", 1, attrs...)
		observability.Duration(ctx, "processor.run", time.Since(started), attrs...)
		observability.End(span, opErr)
	}()
	if err := checkContext(ctx); err != nil {
		opErr = err
		return "", err
	}
	if len(steps) == 0 {
		out, err := copyToTemp(ctx, srcPath, tmpDir)
		if err != nil {
			opErr = err
		}
		return out, err
	}

	// classify steps: split into contiguous streamable segments
	// separated by non-streamable steps
	segments := classifyPipeline(steps)

	// current holds the path to the intermediate file being processed.
	current := srcPath
	// tempFiles tracks all intermediate outputs for cleanup on error.
	// On success, the final output is removed from the list so only
	// true intermediates are deleted.
	var tempFiles []string
	success := false
	defer func() {
		if success {
			return
		}
		for _, f := range tempFiles {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				slog.Warn("processor: failed to clean up temp file", "path", f, "error", err)
			}
		}
	}()

	for _, seg := range segments {
		var nextPath string
		var err error

		if err := checkContext(ctx); err != nil {
			opErr = err
			return "", err
		}
		if seg.streamable {
			nextPath, err = runStreamableSegment(ctx, seg.steps, current, tmpDir)
		} else {
			nextPath, err = runBytesSegment(ctx, seg.steps, current, tmpDir)
		}
		if err != nil {
			if nextPath != "" {
				_ = os.Remove(nextPath)
			}
			opErr = err
			return "", err
		}
		if err := checkContext(ctx); err != nil {
			if nextPath != "" {
				_ = os.Remove(nextPath)
			}
			opErr = err
			return "", err
		}

		// Track the new output immediately so it is cleaned up if a
		// later segment fails. The source file is never ours to remove.
		tempFiles = append(tempFiles, nextPath)
		current = nextPath
	}

	// Remove the final result from the cleanup list — the caller owns it.
	success = true
	final := current
	for _, f := range tempFiles {
		if f != final {
			if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
				slog.Warn("processor: failed to clean up intermediate file", "path", f, "error", err)
			}
		}
	}

	return final, nil
}

type pipelineSegment struct {
	steps      []config.ProcessorStep
	streamable bool
}

func classifyPipeline(steps []config.ProcessorStep) []pipelineSegment {
	var segments []pipelineSegment
	var current []config.ProcessorStep
	currentStreamable := false

	for i, step := range steps {
		name := step.Name
		if name == "" {
			name = "passthrough"
		}
		_, isStream := streamRegistry[name]

		if i == 0 {
			currentStreamable = isStream
			current = append(current, step)
			continue
		}

		if isStream == currentStreamable {
			current = append(current, step)
		} else {
			segments = append(segments, pipelineSegment{
				steps:      current,
				streamable: currentStreamable,
			})
			current = []config.ProcessorStep{step}
			currentStreamable = isStream
		}
	}
	if len(current) > 0 {
		segments = append(segments, pipelineSegment{
			steps:      current,
			streamable: currentStreamable,
		})
	}
	return segments
}

// runStreamableSegment chains streaming processors and writes the output to a temp file.
func runStreamableSegment(ctx context.Context, steps []config.ProcessorStep, srcPath, tmpDir string) (string, error) {
	started := time.Now()
	defer func() {
		observability.Observe(ctx, "processor.stream.segment", 1, 0, time.Since(started), attribute.String("processor.segment", "streamable"))
	}()
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("open source: %w", err)
	}

	// Wrap with context-aware reader so the pipeline aborts on cancellation.
	var reader io.Reader = contextReader{ctx: ctx, r: f}
	var closers []io.Closer
	closers = append(closers, f)

	defer func() {
		for i := len(closers) - 1; i >= 0; i-- {
			_ = closers[i].Close()
		}
	}()

	for _, step := range steps {
		if err := checkContext(ctx); err != nil {
			return "", err
		}
		name := step.Name
		if name == "" {
			name = "passthrough"
		}
		fn := streamRegistry[name]
		if fn == nil {
			return "", fmt.Errorf("stream processor %q not found", name)
		}
		stepStarted := time.Now()
		next, err := fn(reader, step.Args)
		if err != nil {
			observability.Observe(ctx, "processor.stream.step", 1, 0, time.Since(stepStarted), attribute.String("processor.step", name), attribute.String("processor.status", "error"))
			return "", fmt.Errorf("%s: %w", name, err)
		}
		observability.Observe(ctx, "processor.stream.step", 1, 0, time.Since(stepStarted), attribute.String("processor.step", name), attribute.String("processor.status", "ok"))
		if c, ok := next.(io.Closer); ok && next != reader {
			closers = append(closers, c)
		}
		reader = next
	}

	return writeReaderToTemp(ctx, reader, tmpDir)
}

// runBytesSegment reads the source into memory, runs the []byte pipeline,
// and writes the result to a temp file.
func runBytesSegment(ctx context.Context, steps []config.ProcessorStep, srcPath, tmpDir string) (string, error) {
	started := time.Now()
	defer func() {
		observability.Observe(ctx, "processor.stream.segment", 1, 0, time.Since(started), attribute.String("processor.segment", "bytes"))
	}()
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read source: %w", err)
	}
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	out, err := Run(ctx, steps, data)
	if err != nil {
		return "", err
	}
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	return writeBytesToTemp(ctx, out, tmpDir)
}

func writeReaderToTemp(ctx context.Context, r io.Reader, tmpDir string) (string, error) {
	started := time.Now()
	defer func() {
		observability.Count(observability.BackgroundContext(), "processor.temp.writes", 1, attribute.String("processor.temp.kind", "stream"))
		observability.Duration(observability.BackgroundContext(), "processor.temp.write", time.Since(started), attribute.String("processor.temp.kind", "stream"))
	}()
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(tmpDir, "proc-stream-*.tmp")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	_, err = io.Copy(tmp, contextReader{ctx: ctx, r: r})
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := checkContext(ctx); err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func writeBytesToTemp(ctx context.Context, data []byte, tmpDir string) (string, error) {
	started := time.Now()
	defer func() {
		observability.Count(observability.BackgroundContext(), "processor.temp.writes", 1, attribute.String("processor.temp.kind", "bytes"))
		observability.Duration(observability.BackgroundContext(), "processor.temp.write", time.Since(started), attribute.String("processor.temp.kind", "bytes"))
	}()
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(tmpDir, "proc-bytes-*.tmp")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := checkContext(ctx); err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func copyToTemp(ctx context.Context, srcPath, tmpDir string) (string, error) {
	started := time.Now()
	defer func() {
		observability.Count(observability.BackgroundContext(), "processor.temp.writes", 1, attribute.String("processor.temp.kind", "copy"))
		observability.Duration(observability.BackgroundContext(), "processor.temp.write", time.Since(started), attribute.String("processor.temp.kind", "copy"))
	}()
	if err := checkContext(ctx); err != nil {
		return "", err
	}
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return "", err
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = src.Close() }()

	tmp, err := os.CreateTemp(tmpDir, "proc-copy-*.tmp")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	_, err = io.Copy(tmp, contextReader{ctx: ctx, r: src})
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := checkContext(ctx); err != nil {
		_ = tmp.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// RunStreamToFile is a convenience wrapper that writes the result to a specific
// destination path instead of a temp file. It uses RunStream internally.
func RunStreamToFile(ctx context.Context, steps []config.ProcessorStep, srcPath, dstPath, tmpDir string) error {
	resultPath, err := RunStream(ctx, steps, srcPath, tmpDir)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(resultPath) }()

	// atomic rename if same filesystem, otherwise copy
	if err := checkContext(ctx); err != nil {
		return err
	}
	dir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := os.Rename(resultPath, dstPath); err != nil {
		// cross-device: copy instead
		return copyFile(ctx, resultPath, dstPath)
	}
	return nil
}

func copyFile(ctx context.Context, src, dst string) error {
	started := time.Now()
	var bytes int64
	defer func() {
		observability.Observe(observability.BackgroundContext(), "file.copy", 1, bytes, time.Since(started))
	}()
	if err := checkContext(ctx); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if err := checkContext(ctx); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	n, err := io.Copy(out, contextReader{ctx: ctx, r: in})
	bytes = n
	if err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := checkContext(ctx); err != nil {
		_ = out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}
