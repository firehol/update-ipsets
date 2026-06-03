package iprange

import (
	"context"
	"fmt"
	"io"
	"strings"
)

func runCLIV6(ctx context.Context, stdout, stderr io.Writer, stdin io.Reader, args []string) int {
	runner := newCLIV6Runner(stdout, stderr, stdin)
	if code, done := runner.parseArgs(ctx, args); done {
		return code
	}
	return runner.execute()
}

func (r *cliV6Runner) parseArgs(ctx context.Context, args []string) (int, bool) {
	for i := 0; i < len(args); i++ {
		next, code, done := r.parseArg(ctx, args, i)
		if done {
			return code, true
		}
		i = next
	}
	if len(r.before) == 0 {
		set, err := ParseReader6(ctx, DefaultName, r.stdin, r.parseOpts)
		if err != nil {
			return cliError(r.stderr, err), true
		}
		r.before = append(r.before, set)
	}
	return 0, false
}

func (r *cliV6Runner) parseArg(ctx context.Context, args []string, i int) (int, int, bool) {
	return parseCLIArg(ctx, args, i, r)
}

func (r *cliV6Runner) parseAlias(args []string, i int) (int, int, bool) {
	if i+1 >= len(args) {
		return i, cliErrorText(r.stderr, "missing alias after 'as'"), true
	}
	target := &r.before
	if r.readSecond {
		target = &r.after
	}
	if len(*target) == 0 {
		return i, cliErrorText(r.stderr, "alias requires a previous input"), true
	}
	(*target)[len(*target)-1].Name = args[i+1]
	return i + 1, 0, false
}

func (r *cliV6Runner) parseMinPrefix(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--min-prefix")
	if !ok {
		return i, 1, true
	}
	prefix, err := ParsePrefix6(value)
	if err != nil || prefix < 1 || prefix > 128 {
		_, _ = fmt.Fprintf(r.stderr, "iprange: invalid --min-prefix value %q\n", value)
		return next, 1, true
	}
	for j := 0; j < prefix; j++ {
		r.printOpts.PrefixesEnabled[j] = false
	}
	return next, 0, false
}

func (r *cliV6Runner) parsePrefixes(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--prefixes")
	if !ok {
		return i, 1, true
	}
	for j := 0; j < 129; j++ {
		r.printOpts.PrefixesEnabled[j] = false
	}
	for _, part := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' }) {
		prefix, err := ParsePrefix6(part)
		if err != nil || prefix <= 0 || prefix > 128 {
			_, _ = fmt.Fprintf(r.stderr, "iprange: invalid prefix %q\n", part)
			return next, 1, true
		}
		r.printOpts.PrefixesEnabled[prefix] = true
	}
	r.printOpts.PrefixesEnabled[128] = true
	return next, 0, false
}

func (r *cliV6Runner) parseDefaultPrefix(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--default-prefix")
	if !ok {
		return i, 1, true
	}
	prefix, err := ParsePrefix6(value)
	if err != nil {
		_, _ = fmt.Fprintf(r.stderr, "iprange: invalid --default-prefix value %q\n", value)
		return next, 1, true
	}
	r.parseOpts.DefaultPrefix = prefix
	return next, 0, false
}

func (r *cliV6Runner) parseReduceFactor(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--ipset-reduce")
	if !ok {
		return i, 1, true
	}
	reduceFactor, ok := cliIntValue(value)
	if !ok || reduceFactor < 0 {
		_, _ = fmt.Fprintf(r.stderr, "iprange: invalid reduce factor %q\n", value)
		return next, 1, true
	}
	r.reduceFactor = reduceFactor + 100
	r.mode = modeReduce
	return next, 0, false
}

func (r *cliV6Runner) parseReduceEntries(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--reduce-entries")
	if !ok {
		return i, 1, true
	}
	reduceEntries, ok := cliIntValue(value)
	if !ok || reduceEntries < 0 {
		_, _ = fmt.Fprintf(r.stderr, "iprange: invalid reduce entries %q\n", value)
		return next, 1, true
	}
	r.reduceEntries = reduceEntries
	r.mode = modeReduce
	return next, 0, false
}

func (r *cliV6Runner) parsePrintPrefix(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--print-prefix")
	if !ok {
		return i, 1, true
	}
	r.printOpts.PrintPrefixIPs = value
	r.printOpts.PrintPrefixNets = value
	return next, 0, false
}

func (r *cliV6Runner) parsePrintPrefixIPs(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--print-prefix-ips")
	if !ok {
		return i, 1, true
	}
	r.printOpts.PrintPrefixIPs = value
	return next, 0, false
}

func (r *cliV6Runner) parsePrintPrefixNets(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--print-prefix-nets")
	if !ok {
		return i, 1, true
	}
	r.printOpts.PrintPrefixNets = value
	return next, 0, false
}

func (r *cliV6Runner) parsePrintSuffix(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--print-suffix")
	if !ok {
		return i, 1, true
	}
	r.printOpts.PrintSuffixIPs = value
	r.printOpts.PrintSuffixNets = value
	return next, 0, false
}

func (r *cliV6Runner) parsePrintSuffixIPs(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--print-suffix-ips")
	if !ok {
		return i, 1, true
	}
	r.printOpts.PrintSuffixIPs = value
	return next, 0, false
}

func (r *cliV6Runner) parsePrintSuffixNets(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--print-suffix-nets")
	if !ok {
		return i, 1, true
	}
	r.printOpts.PrintSuffixNets = value
	return next, 0, false
}

func (r *cliV6Runner) parseDNSThreads(args []string, i int) (int, int, bool) {
	value, next, ok := cliOptionValue(r.stderr, args, i, "--dns-threads")
	if !ok {
		return i, 1, true
	}
	threads, ok := cliIntValue(value)
	if !ok || threads < 1 {
		_, _ = fmt.Fprintf(r.stderr, "iprange: invalid dns thread count %q\n", value)
		return next, 1, true
	}
	r.parseOpts.DNSThreads = threads
	return next, 0, false
}

func (r *cliV6Runner) loadInput(ctx context.Context, arg string) (int, bool) {
	sets, err := loadInputArgument6(ctx, arg, r.stdin, r.parseOpts)
	if err != nil {
		return cliError(r.stderr, err), true
	}
	if r.readSecond {
		r.after = append(r.after, sets...)
	} else {
		r.before = append(r.before, sets...)
	}
	return 0, false
}

func (r *cliV6Runner) execute() int {
	switch r.mode {
	case modeCombine, modeReduce:
		merged, err := mergeAll6(r.before)
		if err != nil {
			return cliError(r.stderr, err)
		}
		if r.mode == modeReduce {
			r.printOpts.PrefixesEnabled = Reduce6(merged, r.reduceFactor, r.reduceEntries, r.printOpts.PrefixesEnabled)
		}
		return r.writeSet(merged)
	case modeCommon:
		return r.executeCommon()
	case modeExcludeNext:
		return r.executeExcludeNext()
	case modeDiff:
		return r.executeDiff()
	case modeCompare:
		return r.executeCompare()
	case modeCompareNext:
		return r.executeCompareNext()
	case modeCompareFirst:
		return r.executeCompareFirst()
	case modeCountUniqueMerged:
		return r.executeCountUniqueMerged()
	case modeCountUniqueAll:
		return r.executeCountUniqueAll()
	default:
		return cliErrorText(r.stderr, "unknown mode")
	}
}

func (r *cliV6Runner) writeSet(set *IPSet6) int {
	if err := set.Write6(r.stdout, r.printOpts); err != nil {
		return cliError(r.stderr, err)
	}
	return 0
}

func (r *cliV6Runner) executeCommon() int {
	if len(r.before) < 2 {
		return cliErrorText(r.stderr, "common requires at least two ipsets")
	}
	current := r.before[0]
	for _, next := range r.before[1:] {
		current = Intersect6(current, next)
	}
	return r.writeSet(current)
}

func (r *cliV6Runner) executeExcludeNext() int {
	if len(r.after) == 0 {
		return cliErrorText(r.stderr, "no files given after --exclude-next")
	}
	current, err := mergeAll6(r.before)
	if err != nil {
		return cliError(r.stderr, err)
	}
	for _, exclude := range r.after {
		current = Exclude6(current, exclude)
	}
	return r.writeSet(current)
}

func (r *cliV6Runner) executeDiff() int {
	if len(r.after) == 0 {
		return cliErrorText(r.stderr, "no files given after --diff")
	}
	left, err := mergeAll6(r.before)
	if err != nil {
		return cliError(r.stderr, err)
	}
	right, err := mergeAll6(r.after)
	if err != nil {
		return cliError(r.stderr, err)
	}
	diff := Diff6(left, right)
	if !r.quiet {
		if code := r.writeSet(diff); code != 0 {
			return code
		}
	}
	if !diff.UniqueCount().IsZero() {
		return 1
	}
	return 0
}

func (r *cliV6Runner) executeCompare() int {
	rows, err := CompareAll6(r.before)
	if err != nil {
		return cliError(r.stderr, err)
	}
	r.writeCompareHeader()
	for _, row := range rows {
		_, _ = fmt.Fprintf(r.stdout, "%s,%s,%d,%d,%s,%s,%s,%s\n", row.Name1, row.Name2, row.Entries1, row.Entries2, row.Unique1.String(), row.Unique2.String(), row.CombinedIPs.String(), row.CommonIPs.String())
	}
	return 0
}

func (r *cliV6Runner) executeCompareNext() int {
	rows, err := CompareNext6(r.before, r.after)
	if err != nil {
		return cliError(r.stderr, err)
	}
	r.writeCompareHeader()
	for _, row := range rows {
		_, _ = fmt.Fprintf(r.stdout, "%s,%s,%d,%d,%s,%s,%s,%s\n", row.Name1, row.Name2, row.Entries1, row.Entries2, row.Unique1.String(), row.Unique2.String(), row.CombinedIPs.String(), row.CommonIPs.String())
	}
	return 0
}

func (r *cliV6Runner) writeCompareHeader() {
	if r.header {
		_, _ = fmt.Fprintln(r.stdout, "name1,name2,entries1,entries2,ips1,ips2,combined_ips,common_ips")
	}
}

func (r *cliV6Runner) executeCompareFirst() int {
	rows, err := CompareFirst6(r.before)
	if err != nil {
		return cliError(r.stderr, err)
	}
	if r.header {
		_, _ = fmt.Fprintln(r.stdout, "name,entries,unique_ips,common_ips")
	}
	for _, row := range rows {
		_, _ = fmt.Fprintf(r.stdout, "%s,%d,%s,%s\n", row.Name, row.Entries, row.UniqueIPs.String(), row.CommonIPs.String())
	}
	return 0
}

func (r *cliV6Runner) executeCountUniqueMerged() int {
	row, err := CountUniqueMerged6(r.before)
	if err != nil {
		return cliError(r.stderr, err)
	}
	if r.header {
		_, _ = fmt.Fprintln(r.stdout, "entries,unique_ips")
	}
	_, _ = fmt.Fprintf(r.stdout, "%d,%s\n", row.Entries, row.UniqueIPs.String())
	return 0
}

func (r *cliV6Runner) executeCountUniqueAll() int {
	rows, err := CountUniqueAll6(r.before)
	if err != nil {
		return cliError(r.stderr, err)
	}
	if r.header {
		_, _ = fmt.Fprintln(r.stdout, "name,entries,unique_ips")
	}
	for _, row := range rows {
		_, _ = fmt.Fprintf(r.stdout, "%s,%d,%s\n", row.Name, row.Entries, row.UniqueIPs.String())
	}
	return 0
}
