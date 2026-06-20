package downloader

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
	"github.com/firehol/update-ipsets/pkg/processor"
)

func PrepareCanonicalFeedBody(ctx context.Context, name, output, inputPath string, steps []config.ProcessorStep, tmpDir string, dnsThreads int) ([]byte, *iprange.IPSet, error) {
	processedPath, err := processor.RunStream(ctx, steps, inputPath, tmpDir)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = os.Remove(processedPath) }()

	set, err := ParseProcessedFeedFile(ctx, name, processedPath, dnsThreads)
	if err != nil {
		return nil, nil, err
	}

	body, err := RenderCanonicalFeedBody(set, output)
	if err != nil {
		return nil, nil, err
	}
	return body, set, nil
}

func ParseProcessedFeedFile(ctx context.Context, name, path string, dnsThreads int) (*iprange.IPSet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	reader := newSanitizeReader(f)
	opts := iprange.DefaultParseOptions()
	opts.DefaultPrefix = 32
	opts.DNSThreads = dnsThreads
	return iprange.ParseReader(ctx, name, reader, opts)
}

func ParseCanonicalFeedReader(ctx context.Context, name string, r io.Reader, opts iprange.ParseOptions) (*iprange.IPSet, error) {
	return iprange.ParseReader(ctx, name, r, opts)
}

func ParseCanonicalFeedFile(ctx context.Context, name, path string, dnsThreads int) (*iprange.IPSet, error) {
	opts := iprange.DefaultParseOptions()
	opts.DefaultPrefix = 32
	opts.DNSThreads = dnsThreads
	return ParseCanonicalFeedFileWithOptions(ctx, name, path, opts)
}

func ParseCanonicalFeedFileWithOptions(ctx context.Context, name, path string, opts iprange.ParseOptions) (*iprange.IPSet, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	opts.DefaultPrefix = 32
	return ParseCanonicalFeedReader(ctx, name, f, opts)
}

func RenderCanonicalFeedBody(set *iprange.IPSet, output string) ([]byte, error) {
	if set == nil {
		return nil, fmt.Errorf("nil feed body set")
	}
	var buf bytes.Buffer
	opts := iprange.DefaultPrintOptions()
	switch canonicalOutputFamily(output) {
	case "ipset":
		opts.Format = iprange.PrintSingleIPs
	case "netset":
		opts.Format = iprange.PrintCIDR
	default:
		return nil, fmt.Errorf("unsupported output %q", output)
	}
	if err := set.Write(&buf, opts); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func canonicalOutputFamily(output string) string {
	switch strings.TrimSpace(strings.ToLower(output)) {
	case "ip", "ips", "ipset":
		return "ipset"
	case "net", "nets", "both", "all", "netset":
		return "netset"
	default:
		return output
	}
}

// sanitizeReader normalizes a processed non-canonical stream into a shape that
// iprange can ingest without understanding the original upstream format.
type sanitizeReader struct {
	reader    *bufio.Reader
	buf       []byte
	firstLine bool
}

func newSanitizeReader(r io.Reader) *sanitizeReader {
	return &sanitizeReader{
		reader:    bufio.NewReaderSize(r, 64*1024),
		firstLine: true,
	}
}

func (sr *sanitizeReader) Read(p []byte) (int, error) {
	n := 0
	for n < len(p) {
		if len(sr.buf) > 0 {
			copied := copy(p[n:], sr.buf)
			n += copied
			sr.buf = sr.buf[copied:]
			continue
		}
		line, err := sr.reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			if n > 0 {
				return n, nil
			}
			return 0, err
		}
		if len(line) == 0 {
			if n > 0 {
				return n, nil
			}
			return 0, io.EOF
		}
		if sr.firstLine {
			sr.firstLine = false
			line = bytes.TrimPrefix(line, []byte("\xEF\xBB\xBF"))
		}
		line = bytes.ReplaceAll(line, []byte{'\r'}, nil)
		line = bytes.TrimRight(line, "\n")
		fields := strings.Fields(string(line))
		if len(fields) == 0 {
			if errors.Is(err, io.EOF) {
				continue
			}
			continue
		}
		normalized := strings.Join(fields, " ")
		if normalized == "" || normalized == "0.0.0.0" || strings.HasSuffix(normalized, "/0") {
			if errors.Is(err, io.EOF) {
				continue
			}
			continue
		}
		sr.buf = append(sr.buf[:0], normalized...)
		sr.buf = append(sr.buf, '\n')
		if errors.Is(err, io.EOF) {
			// Keep buffered output and let the next Read() return EOF after draining.
			continue
		}
	}
	return n, nil
}
