package iprange

import (
	"context"
	"fmt"
	"io"
)

type cliArgTarget interface {
	cliStdout() io.Writer
	cliStderr() io.Writer
	cliHasBefore() bool
	cliSetMode(cliMode)
	cliSetReadSecond()
	cliSetFormat(PrintFormat)
	cliSetHeader()
	cliSetQuiet()
	cliDisableCIDRNetwork()
	parseAlias([]string, int) (int, int, bool)
	parseMinPrefix([]string, int) (int, int, bool)
	parsePrefixes([]string, int) (int, int, bool)
	parseDefaultPrefix([]string, int) (int, int, bool)
	parseReduceFactor([]string, int) (int, int, bool)
	parseReduceEntries([]string, int) (int, int, bool)
	parsePrintPrefix([]string, int) (int, int, bool)
	parsePrintPrefixIPs([]string, int) (int, int, bool)
	parsePrintPrefixNets([]string, int) (int, int, bool)
	parsePrintSuffix([]string, int) (int, int, bool)
	parsePrintSuffixIPs([]string, int) (int, int, bool)
	parsePrintSuffixNets([]string, int) (int, int, bool)
	parseDNSThreads([]string, int) (int, int, bool)
	loadInput(context.Context, string) (int, bool)
}

func parseCLIArg(ctx context.Context, args []string, i int, p cliArgTarget) (int, int, bool) {
	arg := args[i]
	switch arg {
	case "as":
		return p.parseAlias(args, i)
	case "--min-prefix":
		return p.parseMinPrefix(args, i)
	case "--prefixes":
		return p.parsePrefixes(args, i)
	case "--default-prefix", "-p":
		return p.parseDefaultPrefix(args, i)
	case "--ipset-reduce", "--reduce-factor":
		return p.parseReduceFactor(args, i)
	case "--ipset-reduce-entries", "--reduce-entries":
		return p.parseReduceEntries(args, i)
	case "--optimize", "--combine", "--merge", "--union", "--union-all", "-J":
		p.cliSetMode(modeCombine)
	case "--common", "--intersect", "--intersect-all":
		p.cliSetMode(modeCommon)
	case "--exclude-next", "--except", "--complement-next", "--complement":
		if !p.cliHasBefore() {
			return i, cliErrorText(p.cliStderr(), "an ipset is needed before --exclude-next"), true
		}
		p.cliSetMode(modeExcludeNext)
		p.cliSetReadSecond()
	case "--diff", "--diff-next":
		if !p.cliHasBefore() {
			return i, cliErrorText(p.cliStderr(), "an ipset is needed before --diff"), true
		}
		p.cliSetMode(modeDiff)
		p.cliSetReadSecond()
	case "--compare":
		p.cliSetMode(modeCompare)
	case "--compare-first":
		p.cliSetMode(modeCompareFirst)
	case "--compare-next":
		if !p.cliHasBefore() {
			return i, cliErrorText(p.cliStderr(), "an ipset is needed before --compare-next"), true
		}
		p.cliSetMode(modeCompareNext)
		p.cliSetReadSecond()
	case "--count-unique", "-C":
		p.cliSetMode(modeCountUniqueMerged)
	case "--count-unique-all":
		p.cliSetMode(modeCountUniqueAll)
	case "--print-ranges", "-j":
		p.cliSetFormat(PrintRanges)
	case "--print-binary":
		p.cliSetFormat(PrintBinary)
	case "--print-single-ips", "-1":
		p.cliSetFormat(PrintSingleIPs)
	case "--print-prefix":
		return p.parsePrintPrefix(args, i)
	case "--print-prefix-ips":
		return p.parsePrintPrefixIPs(args, i)
	case "--print-prefix-nets":
		return p.parsePrintPrefixNets(args, i)
	case "--print-suffix":
		return p.parsePrintSuffix(args, i)
	case "--print-suffix-ips":
		return p.parsePrintSuffixIPs(args, i)
	case "--print-suffix-nets":
		return p.parsePrintSuffixNets(args, i)
	case "--header":
		p.cliSetHeader()
	case "--quiet":
		p.cliSetQuiet()
	case "--dont-fix-network":
		p.cliDisableCIDRNetwork()
	case "--dns-threads":
		return p.parseDNSThreads(args, i)
	case "--dns-silent", "--dns-progress":
		return i, 0, false
	case "--has-compare", "--has-reduce":
		_, _ = fmt.Fprintln(p.cliStderr(), "yes, compare and reduce is present.")
		return i, 0, true
	case "--has-filelist-loading", "--has-directory-loading":
		_, _ = fmt.Fprintln(p.cliStderr(), "yes, @filename and @directory support is present.")
		return i, 0, true
	case "--version":
		_, _ = fmt.Fprintln(p.cliStdout(), "iprange go-dev")
		return i, 0, true
	case "--help", "-h":
		return i, printCLIUsage(p.cliStdout()), true
	default:
		if code, done := p.loadInput(ctx, arg); done {
			return i, code, true
		}
	}
	return i, 0, false
}

func cliOptionValue(stderr io.Writer, args []string, i int, name string) (string, int, bool) {
	if i+1 >= len(args) {
		_, _ = fmt.Fprintf(stderr, "iprange: %s requires a value\n", name)
		return "", i, false
	}
	return args[i+1], i + 1, true
}

func cliIntValue(value string) (int, bool) {
	var out int
	if _, err := fmt.Sscanf(value, "%d", &out); err != nil {
		return 0, false
	}
	return out, true
}

func cliError(stderr io.Writer, err error) int {
	_, _ = fmt.Fprintf(stderr, "iprange: %v\n", err)
	return 1
}

func cliErrorText(stderr io.Writer, text string) int {
	_, _ = fmt.Fprintln(stderr, "iprange: "+text)
	return 1
}
