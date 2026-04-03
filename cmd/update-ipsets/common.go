package main

import (
	"log/slog"
	"os"
)

func newLogger(silent, verbose bool) *slog.Logger {
	level := slog.LevelInfo
	if silent {
		level = slog.LevelError
	}
	if verbose {
		level = slog.LevelDebug
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}
