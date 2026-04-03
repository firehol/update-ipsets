package iprange

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type cliMode int

const (
	modeCombine cliMode = iota + 1
	modeCompare
	modeCompareFirst
	modeCompareNext
	modeCountUniqueMerged
	modeCountUniqueAll
	modeReduce
	modeCommon
	modeExcludeNext
	modeDiff
)

func runCLIV4(ctx context.Context, stdout, stderr io.Writer, stdin io.Reader, args []string) int {
	mode := modeCombine
	printOpts := DefaultPrintOptions()
	parseOpts := DefaultParseOptions()
	parseOpts.DefaultPrefix = 32
	readSecond := false
	header := false
	quiet := false
	reduceFactor := 120
	reduceEntries := 16384

	before := make([]*IPSet, 0)
	after := make([]*IPSet, 0)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "as":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: missing alias after 'as'")
				return 1
			}
			target := &before
			if readSecond {
				target = &after
			}
			if len(*target) == 0 {
				_, _ = fmt.Fprintln(stderr, "iprange: alias requires a previous input")
				return 1
			}
			(*target)[len(*target)-1].Name = args[i+1]
			i++
		case "--min-prefix":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --min-prefix requires a value")
				return 1
			}
			prefix, err := ParsePrefix(args[i+1])
			if err != nil || prefix < 1 || prefix > 32 {
				_, _ = fmt.Fprintf(stderr, "iprange: invalid --min-prefix value %q\n", args[i+1])
				return 1
			}
			for j := 0; j < prefix; j++ {
				printOpts.PrefixesEnabled[j] = false
			}
			i++
		case "--prefixes":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --prefixes requires a value")
				return 1
			}
			for j := 0; j < 32; j++ {
				printOpts.PrefixesEnabled[j] = false
			}
			for _, part := range strings.FieldsFunc(args[i+1], func(r rune) bool { return r == ',' || r == ' ' }) {
				prefix, err := ParsePrefix(part)
				if err != nil || prefix <= 0 || prefix > 32 {
					_, _ = fmt.Fprintf(stderr, "iprange: invalid prefix %q\n", part)
					return 1
				}
				printOpts.PrefixesEnabled[prefix] = true
			}
			printOpts.PrefixesEnabled[32] = true
			i++
		case "--default-prefix", "-p":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --default-prefix requires a value")
				return 1
			}
			prefix, err := ParsePrefix(args[i+1])
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "iprange: invalid --default-prefix value %q\n", args[i+1])
				return 1
			}
			parseOpts.DefaultPrefix = prefix
			i++
		case "--ipset-reduce", "--reduce-factor":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --ipset-reduce requires a value")
				return 1
			}
			var err error
			_, err = fmt.Sscanf(args[i+1], "%d", &reduceFactor)
			if err != nil || reduceFactor < 0 {
				_, _ = fmt.Fprintf(stderr, "iprange: invalid reduce factor %q\n", args[i+1])
				return 1
			}
			reduceFactor += 100
			mode = modeReduce
			i++
		case "--ipset-reduce-entries", "--reduce-entries":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --reduce-entries requires a value")
				return 1
			}
			var err error
			_, err = fmt.Sscanf(args[i+1], "%d", &reduceEntries)
			if err != nil || reduceEntries < 0 {
				_, _ = fmt.Fprintf(stderr, "iprange: invalid reduce entries %q\n", args[i+1])
				return 1
			}
			mode = modeReduce
			i++
		case "--optimize", "--combine", "--merge", "--union", "--union-all", "-J":
			mode = modeCombine
		case "--common", "--intersect", "--intersect-all":
			mode = modeCommon
		case "--exclude-next", "--except", "--complement-next", "--complement":
			if len(before) == 0 {
				_, _ = fmt.Fprintln(stderr, "iprange: an ipset is needed before --exclude-next")
				return 1
			}
			mode = modeExcludeNext
			readSecond = true
		case "--diff", "--diff-next":
			if len(before) == 0 {
				_, _ = fmt.Fprintln(stderr, "iprange: an ipset is needed before --diff")
				return 1
			}
			mode = modeDiff
			readSecond = true
		case "--compare":
			mode = modeCompare
		case "--compare-first":
			mode = modeCompareFirst
		case "--compare-next":
			if len(before) == 0 {
				_, _ = fmt.Fprintln(stderr, "iprange: an ipset is needed before --compare-next")
				return 1
			}
			mode = modeCompareNext
			readSecond = true
		case "--count-unique", "-C":
			mode = modeCountUniqueMerged
		case "--count-unique-all":
			mode = modeCountUniqueAll
		case "--print-ranges", "-j":
			printOpts.Format = PrintRanges
		case "--print-binary":
			printOpts.Format = PrintBinary
		case "--print-single-ips", "-1":
			printOpts.Format = PrintSingleIPs
		case "--print-prefix":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --print-prefix requires a value")
				return 1
			}
			printOpts.PrintPrefixIPs = args[i+1]
			printOpts.PrintPrefixNets = args[i+1]
			i++
		case "--print-prefix-ips":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --print-prefix-ips requires a value")
				return 1
			}
			printOpts.PrintPrefixIPs = args[i+1]
			i++
		case "--print-prefix-nets":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --print-prefix-nets requires a value")
				return 1
			}
			printOpts.PrintPrefixNets = args[i+1]
			i++
		case "--print-suffix":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --print-suffix requires a value")
				return 1
			}
			printOpts.PrintSuffixIPs = args[i+1]
			printOpts.PrintSuffixNets = args[i+1]
			i++
		case "--print-suffix-ips":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --print-suffix-ips requires a value")
				return 1
			}
			printOpts.PrintSuffixIPs = args[i+1]
			i++
		case "--print-suffix-nets":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --print-suffix-nets requires a value")
				return 1
			}
			printOpts.PrintSuffixNets = args[i+1]
			i++
		case "--header":
			header = true
		case "--quiet":
			quiet = true
		case "--dont-fix-network":
			parseOpts.UseCIDRNetwork = false
		case "--dns-threads":
			if i+1 >= len(args) {
				_, _ = fmt.Fprintln(stderr, "iprange: --dns-threads requires a value")
				return 1
			}
			var err error
			_, err = fmt.Sscanf(args[i+1], "%d", &parseOpts.DNSThreads)
			if err != nil || parseOpts.DNSThreads < 1 {
				_, _ = fmt.Fprintf(stderr, "iprange: invalid dns thread count %q\n", args[i+1])
				return 1
			}
			i++
		case "--dns-silent", "--dns-progress":
			continue
		case "--has-compare", "--has-reduce":
			_, _ = fmt.Fprintln(stderr, "yes, compare and reduce is present.")
			return 0
		case "--has-filelist-loading", "--has-directory-loading":
			_, _ = fmt.Fprintln(stderr, "yes, @filename and @directory support is present.")
			return 0
		case "--version":
			_, _ = fmt.Fprintln(stdout, "iprange go-dev")
			return 0
		case "--help", "-h":
			return printCLIUsage(stdout)
		default:
			set, err := loadInputArgument(ctx, arg, stdin, parseOpts)
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
				return 1
			}
			if readSecond {
				after = append(after, set...)
			} else {
				before = append(before, set...)
			}
		}
	}

	if len(before) == 0 {
		set, err := ParseReader(ctx, DefaultName, stdin, parseOpts)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		before = append(before, set)
	}

	switch mode {
	case modeCombine, modeReduce:
		merged, err := mergeAll(before)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		if mode == modeReduce {
			printOpts.PrefixesEnabled = Reduce(merged, reduceFactor, reduceEntries, printOpts.PrefixesEnabled)
		}
		if err := merged.Write(stdout, printOpts); err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		return 0
	case modeCommon:
		if len(before) < 2 {
			_, _ = fmt.Fprintln(stderr, "iprange: common requires at least two ipsets")
			return 1
		}
		current := before[0]
		for _, next := range before[1:] {
			current = Intersect(current, next)
		}
		if err := current.Write(stdout, printOpts); err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		return 0
	case modeExcludeNext:
		if len(after) == 0 {
			_, _ = fmt.Fprintln(stderr, "iprange: no files given after --exclude-next")
			return 1
		}
		current, err := mergeAll(before)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		for _, exclude := range after {
			current = Exclude(current, exclude)
		}
		if err := current.Write(stdout, printOpts); err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		return 0
	case modeDiff:
		if len(after) == 0 {
			_, _ = fmt.Fprintln(stderr, "iprange: no files given after --diff")
			return 1
		}
		left, err := mergeAll(before)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		right, err := mergeAll(after)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		diff := Diff(left, right)
		if !quiet {
			if err := diff.Write(stdout, printOpts); err != nil {
				_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
				return 1
			}
		}
		if diff.UniqueCount() > 0 {
			return 1
		}
		return 0
	case modeCompare:
		rows, err := CompareAll(before)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		if header {
			_, _ = fmt.Fprintln(stdout, "name1,name2,entries1,entries2,ips1,ips2,combined_ips,common_ips")
		}
		for _, row := range rows {
			_, _ = fmt.Fprintf(stdout, "%s,%s,%d,%d,%d,%d,%d,%d\n", row.Name1, row.Name2, row.Entries1, row.Entries2, row.Unique1, row.Unique2, row.CombinedIPs, row.CommonIPs)
		}
		return 0
	case modeCompareNext:
		rows, err := CompareNext(before, after)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		if header {
			_, _ = fmt.Fprintln(stdout, "name1,name2,entries1,entries2,ips1,ips2,combined_ips,common_ips")
		}
		for _, row := range rows {
			_, _ = fmt.Fprintf(stdout, "%s,%s,%d,%d,%d,%d,%d,%d\n", row.Name1, row.Name2, row.Entries1, row.Entries2, row.Unique1, row.Unique2, row.CombinedIPs, row.CommonIPs)
		}
		return 0
	case modeCompareFirst:
		rows, err := CompareFirst(before)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		if header {
			_, _ = fmt.Fprintln(stdout, "name,entries,unique_ips,common_ips")
		}
		for _, row := range rows {
			_, _ = fmt.Fprintf(stdout, "%s,%d,%d,%d\n", row.Name, row.Entries, row.UniqueIPs, row.CommonIPs)
		}
		return 0
	case modeCountUniqueMerged:
		row, err := CountUniqueMerged(before)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		if header {
			_, _ = fmt.Fprintln(stdout, "entries,unique_ips")
		}
		_, _ = fmt.Fprintf(stdout, "%d,%d\n", row.Entries, row.UniqueIPs)
		return 0
	case modeCountUniqueAll:
		rows, err := CountUniqueAll(before)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
			return 1
		}
		if header {
			_, _ = fmt.Fprintln(stdout, "name,entries,unique_ips")
		}
		for _, row := range rows {
			_, _ = fmt.Fprintf(stdout, "%s,%d,%d\n", row.Name, row.Entries, row.UniqueIPs)
		}
		return 0
	default:
		_, _ = fmt.Fprintln(stderr, "iprange: unknown mode")
		return 1
	}
}
