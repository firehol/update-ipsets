package scheduler

import (
	"sort"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
	"github.com/firehol/update-ipsets/pkg/runreason"
)

type QueueFeed struct {
	Name           string                      `json:"name"`
	Kind           string                      `json:"kind,omitempty"`
	Reason         runreason.Reason            `json:"reason,omitempty"`
	QueuedAt       time.Time                   `json:"queued_at,omitempty"`
	EnqueueSeq     uint64                      `json:"enqueue_seq,omitempty"`
	Status         string                      `json:"status,omitempty"`
	StatusLabel    string                      `json:"status_label,omitempty"`
	ProblemClass   engine.OperatorProblemClass `json:"problem_class,omitempty"`
	Detail         string                      `json:"detail,omitempty"`
	Blocked        bool                        `json:"blocked,omitempty"`
	BlockedParents []string                    `json:"blocked_parents,omitempty"`
}

type ActiveQueueFeed struct {
	Name         string                      `json:"name"`
	Kind         string                      `json:"kind,omitempty"`
	Reason       runreason.Reason            `json:"reason,omitempty"`
	StartedAt    time.Time                   `json:"started_at,omitempty"`
	Status       string                      `json:"status,omitempty"`
	StatusLabel  string                      `json:"status_label,omitempty"`
	ProblemClass engine.OperatorProblemClass `json:"problem_class,omitempty"`
	Detail       string                      `json:"detail,omitempty"`
}

type ActivitySnapshot struct {
	DownloadWaiting         []QueueFeed        `json:"download_waiting,omitempty"`
	DownloadActive          []ActiveQueueFeed  `json:"download_active,omitempty"`
	DownloadRefetchPending  []QueueFeed        `json:"download_refetch_pending,omitempty"`
	ProcessingWaiting       []QueueFeed        `json:"processing_waiting,omitempty"`
	ProcessingActive        []ActiveQueueFeed  `json:"processing_active,omitempty"`
	ProcessingDeferred      []QueueFeed        `json:"processing_deferred,omitempty"`
	RecentHealthTransitions []HealthTransition `json:"recent_health_transitions,omitempty"`
}

type HealthTransition struct {
	Feed      string    `json:"feed"`
	FromClass string    `json:"from_class"`
	ToClass   string    `json:"to_class"`
	At        time.Time `json:"at"`
}

type queuedWorkKind string

const (
	queuedWorkKindNormal            queuedWorkKind = "normal"
	queuedWorkKindRecoveredArtifact queuedWorkKind = "recovered_artifact"
)

type queuedWork struct {
	Name       string
	Kind       queuedWorkKind
	Reason     runreason.Reason
	QueuedAt   time.Time
	EnqueueSeq uint64
	ForceRun   bool
	Immediate  bool
	Promote    []string
}

type queueStatusView struct {
	Status       string
	StatusLabel  string
	ProblemClass engine.OperatorProblemClass
	Detail       string
}

func queueSnapshotFromMap(items map[string]queuedWork, lookup func(name string) queueStatusView) []QueueFeed {
	return queueSnapshotFromMapFiltered(items, nil, lookup)
}

func queueSnapshotFromMapFiltered(items map[string]queuedWork, include func(name string) bool, lookup func(name string) queueStatusView) []QueueFeed {
	if len(items) == 0 {
		return nil
	}
	out := make([]QueueFeed, 0, len(items))
	for _, item := range items {
		if include != nil && !include(item.Name) {
			continue
		}
		status := queueStatusView{}
		if lookup != nil {
			status = lookup(item.Name)
		}
		out = append(out, QueueFeed{
			Name:         item.Name,
			Kind:         string(item.Kind),
			Reason:       item.Reason,
			QueuedAt:     item.QueuedAt,
			EnqueueSeq:   item.EnqueueSeq,
			Status:       status.Status,
			StatusLabel:  status.StatusLabel,
			ProblemClass: status.ProblemClass,
			Detail:       status.Detail,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].QueuedAt.Equal(out[j].QueuedAt) {
			if out[i].EnqueueSeq != out[j].EnqueueSeq {
				if out[i].EnqueueSeq == 0 {
					return false
				}
				if out[j].EnqueueSeq == 0 {
					return true
				}
				return out[i].EnqueueSeq < out[j].EnqueueSeq
			}
			return out[i].Name < out[j].Name
		}
		if out[i].QueuedAt.IsZero() {
			return false
		}
		if out[j].QueuedAt.IsZero() {
			return true
		}
		return out[i].QueuedAt.Before(out[j].QueuedAt)
	})
	return out
}

func activeSnapshotFromMap(items map[string]ActiveQueueFeed, lookup func(name string) queueStatusView) []ActiveQueueFeed {
	return activeSnapshotFromMapFiltered(items, nil, lookup)
}

func activeSnapshotFromMapFiltered(items map[string]ActiveQueueFeed, include func(name string) bool, lookup func(name string) queueStatusView) []ActiveQueueFeed {
	if len(items) == 0 {
		return nil
	}
	out := make([]ActiveQueueFeed, 0, len(items))
	for _, item := range items {
		if include != nil && !include(item.Name) {
			continue
		}
		if lookup != nil {
			status := lookup(item.Name)
			item.Status = status.Status
			item.StatusLabel = status.StatusLabel
			item.ProblemClass = status.ProblemClass
			item.Detail = status.Detail
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].Name < out[j].Name
		}
		if out[i].StartedAt.IsZero() {
			return false
		}
		if out[j].StartedAt.IsZero() {
			return true
		}
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out
}
