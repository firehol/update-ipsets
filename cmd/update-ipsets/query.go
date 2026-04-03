package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func runQuery(args []string) int {
	fs := flag.NewFlagSet("query", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "config path")
	setExpr := fs.String("set", "", "composed set expression (for example: set1 + set2 - set3)")
	ipArg := fs.String("ip", "", "IP to test when using --set")
	format := fs.String("format", "cidr", "output format for --set without an IP: cidr|range|single")
	silent := fs.Bool("silent", false, "errors only")
	verbose := fs.Bool("verbose", false, "verbose logging")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *setExpr == "" && fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "update-ipsets query: missing IP argument")
		return 2
	}

	eng, err := engine.New(*configPath, newLogger(*silent, *verbose))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if *setExpr != "" {
		include, exclude := parseSetExpression(*setExpr)
		if len(include) == 0 {
			fmt.Fprintln(os.Stderr, "update-ipsets query: --set must include at least one set")
			return 2
		}
		ip := *ipArg
		if ip == "" && fs.NArg() == 1 {
			ip = fs.Arg(0)
		}
		data, err := eng.Compose(context.Background(), include, exclude, *format)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if ip == "" {
			_, _ = os.Stdout.Write(data)
			return 0
		}
		set, err := iprange.ParseReader(context.Background(), "composed", bytes.NewReader(data), iprange.DefaultParseOptions())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		ipv4, err := iprange.ParseIPv4Token(ip)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if set.Contains(ipv4) {
			_, _ = fmt.Fprintf(os.Stdout, "%s,included\n", ip)
			return 0
		}
		_, _ = fmt.Fprintf(os.Stdout, "%s,excluded\n", ip)
		return 1
	}
	matches, err := eng.QueryIP(context.Background(), fs.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	for _, match := range matches {
		_, _ = fmt.Fprintf(os.Stdout, "%s,%s,%s,%s\n", match.Name, match.File, match.Category, match.Maintainer)
	}
	return 0
}

func parseSetExpression(raw string) ([]string, []string) {
	normalized := strings.NewReplacer("+", " + ", "-", " - ").Replace(raw)
	fields := strings.Fields(normalized)
	include := make([]string, 0)
	exclude := make([]string, 0)
	currentExclude := false
	for _, field := range fields {
		switch field {
		case "+":
			currentExclude = false
		case "-":
			currentExclude = true
		default:
			if currentExclude {
				exclude = append(exclude, field)
			} else {
				include = append(include, field)
			}
		}
	}
	return include, exclude
}
