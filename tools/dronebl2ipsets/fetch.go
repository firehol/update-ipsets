package dronebl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var ErrMissingPassword = errors.New("missing DroneBL rsync password")

type FetchOptions struct {
	RsyncBin string
	RsyncURL string
	DataDir  string
}

func FetchBuildzone(ctx context.Context, opts FetchOptions) error {
	if opts.RsyncBin == "" {
		opts.RsyncBin = "rsync"
	}
	if opts.RsyncURL == "" {
		return fmt.Errorf("rsync URL is required")
	}
	if opts.DataDir == "" {
		return fmt.Errorf("data directory is required")
	}
	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	password := os.Getenv("DRONEBL_RSYNC_PASSWORD")
	if password == "" {
		password = os.Getenv("RSYNC_PASSWORD")
	}
	if password == "" {
		return ErrMissingPassword
	}

	cmd := exec.CommandContext(ctx, opts.RsyncBin, "-HaSPvz", opts.RsyncURL, opts.DataDir+"/")
	cmd.Env = withRsyncPassword(os.Environ(), password)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync DroneBL buildzone: %w", err)
	}
	return nil
}

func withRsyncPassword(env []string, password string) []string {
	out := env[:0]
	for _, item := range env {
		if len(item) >= len("RSYNC_PASSWORD=") && item[:len("RSYNC_PASSWORD=")] == "RSYNC_PASSWORD=" {
			continue
		}
		out = append(out, item)
	}
	return append(out, "RSYNC_PASSWORD="+password)
}
