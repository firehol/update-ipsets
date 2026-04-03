package runreason

// Reason explains why a run or a feed-processing attempt happened.
// It is intentionally operator-facing: it answers "why did this run?"
// without leaking endpoint or transport details into the UI.
type Reason string

const (
	ReasonUnknown                   Reason = ""
	ReasonScheduledDue              Reason = "scheduled_due"
	ReasonManualRun                 Reason = "manual_run"
	ReasonManualRecheck             Reason = "manual_recheck"
	ReasonManualReprocess           Reason = "manual_reprocess"
	ReasonStartupIntegrityReprocess Reason = "startup_integrity_reprocess"
	ReasonIntegrityReprocess        Reason = "integrity_reprocess"
	ReasonDependencyUpdate          Reason = "dependency_update"
	ReasonProviderDefaults          Reason = "provider_defaults_update"
)

func (r Reason) Valid() bool {
	switch r {
	case ReasonUnknown,
		ReasonScheduledDue,
		ReasonManualRun,
		ReasonManualRecheck,
		ReasonManualReprocess,
		ReasonStartupIntegrityReprocess,
		ReasonIntegrityReprocess,
		ReasonDependencyUpdate,
		ReasonProviderDefaults:
		return true
	default:
		return false
	}
}

func (r Reason) String() string {
	return string(r)
}
