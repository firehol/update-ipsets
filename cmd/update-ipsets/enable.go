package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/firehol/update-ipsets/pkg/engine"
)

func runEnable(args []string) int {
	fs := flag.NewFlagSet("enable", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "config path")
	all := fs.Bool("all", false, "enable all known sources and merges")
	disable := fs.Bool("disable", false, "disable instead of enable")
	silent := fs.Bool("silent", false, "errors only")
	verbose := fs.Bool("verbose", false, "verbose logging")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if !*all && fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "update-ipsets enable: provide one or more names or use --all")
		return 2
	}

	eng, err := engine.New(*configPath, newLogger(*silent, *verbose))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *disable {
		err = eng.Disable(fs.Args(), *all)
	} else {
		err = eng.Enable(fs.Args(), *all)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
