package main

import (
	"context"
	"fmt"
	"os"

	"github.com/firehol/update-ipsets/pkg/iprange"
)

// version is set at build time via -ldflags "-X main.version=...".
// Falls back to "dev" when built without ldflags (e.g. plain `go build`).
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		return usage(os.Stderr)
	}

	switch args[0] {
	case "iprange":
		return iprange.RunCLI(context.Background(), os.Stdout, os.Stderr, os.Stdin, args[1:])
	case "query":
		return runQuery(args[1:])
	case "enable":
		return runEnable(args[1:])
	case "cache-merge":
		return runCacheMerge(args[1:])
	case "daemon":
		return runDaemon(args[1:])
	case "version", "--version":
		_, _ = fmt.Fprintf(os.Stdout, "update-ipsets %s\n", version)
		return 0
	case "help", "--help", "-h":
		return usage(os.Stdout)
	default:
		fmt.Fprintf(os.Stderr, "update-ipsets: unknown subcommand %q\n", args[0])
		return usage(os.Stderr)
	}
}

func usage(out *os.File) int {
	_, _ = fmt.Fprintln(out, "Usage: update-ipsets <command> [options]")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Commands:")
	_, _ = fmt.Fprintln(out, "  iprange   standalone iprange-compatible mode")
	_, _ = fmt.Fprintln(out, "  query     query which lists contain an IP")
	_, _ = fmt.Fprintln(out, "  enable    enable one or more sources")
	_, _ = fmt.Fprintln(out, "  daemon    scheduler + web server + API + admin")
	_, _ = fmt.Fprintln(out, "  version   print version")
	return 1
}
