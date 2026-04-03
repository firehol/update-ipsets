package scheduler

import (
	"slices"
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/engine"
)

func runSchedulerStyleOnce(t *testing.T, eng *engine.Engine, opts engine.RunOptions) (*engine.Report, error) {
	t.Helper()

	requested := opts.Selected
	if len(requested) == 0 {
		requested = config.SortedSourceNames(eng.Config())
	}

	force := opts.Recheck || opts.Reprocess
	processSet := make(map[string]struct{}, len(requested))
	promoteSet := make(map[string]struct{}, len(requested))

	for _, name := range requested {
		if name == "" {
			continue
		}
		if !eng.IsDownloadable(name) {
			processSet[name] = struct{}{}
			continue
		}
		decision, err := eng.FetchAndStage(t.Context(), name, force, opts.EnableAll)
		if err != nil {
			continue
		}
		for _, processName := range decision.ProcessingNames {
			if processName != "" {
				processSet[processName] = struct{}{}
			}
		}
		for _, promoteName := range decision.PromoteNames {
			if promoteName != "" {
				promoteSet[promoteName] = struct{}{}
			}
		}
	}

	if len(processSet) == 0 {
		return &engine.Report{
			Messages: map[string]string{},
			Statuses: map[string]string{},
		}, nil
	}

	processNames := make([]string, 0, len(processSet))
	for _, name := range config.SortedSourceNames(eng.Config()) {
		if _, ok := processSet[name]; ok {
			processNames = append(processNames, name)
		}
	}
	slices.Sort(processNames)

	runOpts := opts
	runOpts.Selected = processNames
	if len(promoteSet) > 0 {
		promoteNames := make([]string, 0, len(promoteSet))
		for name := range promoteSet {
			promoteNames = append(promoteNames, name)
		}
		slices.Sort(promoteNames)
		runOpts.BeforePublish = func(report *engine.Report) error {
			return eng.PromoteCommittedDownloads(promoteNames)
		}
	}

	report, err := eng.RunOnce(t.Context(), runOpts)
	if err != nil {
		return report, err
	}

	return report, nil
}
