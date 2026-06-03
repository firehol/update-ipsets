package iprange

import "io"

type cliBaseRunner struct {
	stdout        io.Writer
	stderr        io.Writer
	stdin         io.Reader
	mode          cliMode
	parseOpts     ParseOptions
	readSecond    bool
	header        bool
	quiet         bool
	reduceFactor  int
	reduceEntries int
}

func (r *cliBaseRunner) cliStdout() io.Writer {
	return r.stdout
}

func (r *cliBaseRunner) cliStderr() io.Writer {
	return r.stderr
}

func (r *cliBaseRunner) cliSetMode(mode cliMode) {
	r.mode = mode
}

func (r *cliBaseRunner) cliSetReadSecond() {
	r.readSecond = true
}

func (r *cliBaseRunner) cliSetHeader() {
	r.header = true
}

func (r *cliBaseRunner) cliSetQuiet() {
	r.quiet = true
}

func (r *cliBaseRunner) cliDisableCIDRNetwork() {
	r.parseOpts.UseCIDRNetwork = false
}

type cliV4Runner struct {
	cliBaseRunner
	printOpts PrintOptions
	before    []*IPSet
	after     []*IPSet
}

func newCLIV4Runner(stdout, stderr io.Writer, stdin io.Reader) *cliV4Runner {
	parseOpts := DefaultParseOptions()
	parseOpts.DefaultPrefix = 32
	return &cliV4Runner{
		cliBaseRunner: cliBaseRunner{
			stdout:        stdout,
			stderr:        stderr,
			stdin:         stdin,
			mode:          modeCombine,
			parseOpts:     parseOpts,
			reduceFactor:  120,
			reduceEntries: 16384,
		},
		printOpts: DefaultPrintOptions(),
		before:    make([]*IPSet, 0),
		after:     make([]*IPSet, 0),
	}
}

func (r *cliV4Runner) cliHasBefore() bool {
	return len(r.before) > 0
}

func (r *cliV4Runner) cliSetFormat(format PrintFormat) {
	r.printOpts.Format = format
}

type cliV6Runner struct {
	cliBaseRunner
	printOpts PrintOptions6
	before    []*IPSet6
	after     []*IPSet6
}

func newCLIV6Runner(stdout, stderr io.Writer, stdin io.Reader) *cliV6Runner {
	return &cliV6Runner{
		cliBaseRunner: cliBaseRunner{
			stdout:        stdout,
			stderr:        stderr,
			stdin:         stdin,
			mode:          modeCombine,
			parseOpts:     DefaultParseOptions6(),
			reduceFactor:  120,
			reduceEntries: 16384,
		},
		printOpts: DefaultPrintOptions6(),
		before:    make([]*IPSet6, 0),
		after:     make([]*IPSet6, 0),
	}
}

func (r *cliV6Runner) cliHasBefore() bool {
	return len(r.before) > 0
}

func (r *cliV6Runner) cliSetFormat(format PrintFormat) {
	r.printOpts.Format = format
}
