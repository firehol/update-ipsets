package dronebl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	Format          = "dronebl_buildzone_class"
	ListsAttribute  = "dronebl_lists"
	DefaultRsyncURL = "rsync://firehol@rsync.dronebl.org/dronebl/"
)

type Options struct {
	WorkDir       string
	OutputDir     string
	BuildzonePath string
	RsyncURL      string
	Timeout       time.Duration
	SkipFetch     bool
	Specs         []OutputSpec
}

type Report struct {
	BuildzonePath string
	Warnings      []string
	Outputs       []OutputReport
}

type OutputReport struct {
	Name      string
	Entries   int
	UniqueIPs uint64
}

func Update(ctx context.Context, opts Options) (*Report, error) {
	if opts.WorkDir == "" {
		return nil, fmt.Errorf("work directory is required")
	}
	if opts.OutputDir == "" {
		return nil, fmt.Errorf("output directory is required")
	}
	if len(opts.Specs) == 0 {
		return nil, fmt.Errorf("at least one output spec is required")
	}
	if opts.RsyncURL == "" {
		opts.RsyncURL = DefaultRsyncURL
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Minute
	}
	if opts.BuildzonePath == "" {
		opts.BuildzonePath = filepath.Join(opts.WorkDir, "data", "buildzone")
	}

	runCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	if !opts.SkipFetch {
		if err := FetchBuildzone(runCtx, FetchOptions{
			RsyncURL: opts.RsyncURL,
			DataDir:  filepath.Join(opts.WorkDir, "data"),
		}); err != nil {
			return nil, err
		}
	}

	file, err := os.Open(opts.BuildzonePath) // nosemgrep: CLI tool reads the operator-selected DroneBL buildzone file.
	if err != nil {
		return nil, fmt.Errorf("open buildzone: %w", err)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat buildzone: %w", err)
	}

	parsed, err := ParseBuildzone(file)
	if err != nil {
		return nil, err
	}

	outputs := BuildOutputs(parsed, opts.Specs)
	report := &Report{
		BuildzonePath: opts.BuildzonePath,
		Warnings:      append([]string(nil), parsed.Warnings...),
		Outputs:       make([]OutputReport, 0, len(opts.Specs)),
	}
	for _, spec := range opts.Specs {
		set := outputs[spec.Name]
		if err := WriteSourceFile(opts.OutputDir, spec.Name+".source", set, info.ModTime()); err != nil {
			return nil, err
		}
		report.Outputs = append(report.Outputs, OutputReport{
			Name:      spec.Name,
			Entries:   set.Entries(),
			UniqueIPs: set.UniqueCount(),
		})
	}

	select {
	case <-runCtx.Done():
		return nil, runCtx.Err()
	default:
		return report, nil
	}
}
