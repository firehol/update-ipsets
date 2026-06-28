package scheduler

import (
	"sync"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

type MetricsSnapshot struct {
	DownloadEnqueued           int64                          `json:"download_enqueued"`
	DownloadDeferred           int64                          `json:"download_deferred"`
	DownloadStarted            int64                          `json:"download_started"`
	DownloadFinished           int64                          `json:"download_finished"`
	ProcessingEnqueued         int64                          `json:"processing_enqueued"`
	ProcessingRequeued         int64                          `json:"processing_requeued"`
	ProcessingBatchesStarted   int64                          `json:"processing_batches_started"`
	ProcessingBatchesCompleted int64                          `json:"processing_batches_completed"`
	ProcessingItemsStarted     int64                          `json:"processing_items_started"`
	MaxDownloadWaiting         int                            `json:"max_download_waiting"`
	MaxProcessingWaiting       int                            `json:"max_processing_waiting"`
	LastBatchSize              int                            `json:"last_batch_size"`
	LastBatchDurationMS        int64                          `json:"last_batch_duration_ms"`
	SnapshotPersistErrors      int64                          `json:"snapshot_persist_errors,omitempty"`
	RecoveredPanics            int64                          `json:"recovered_panics,omitempty"`
	ActionAdmissionFailures    int64                          `json:"action_admission_failures,omitempty"`
	Degraded                   bool                           `json:"degraded,omitempty"`
	DegradedReason             string                         `json:"degraded_reason,omitempty"`
	DegradedAt                 time.Time                      `json:"degraded_at,omitempty"`
	Operations                 []telemetry.TimingStatSnapshot `json:"operations,omitempty"`
}

type metricsState struct {
	mu sync.Mutex

	downloadEnqueued           int64
	downloadDeferred           int64
	downloadStarted            int64
	downloadFinished           int64
	processingEnqueued         int64
	processingRequeued         int64
	processingBatchesStarted   int64
	processingBatchesCompleted int64
	processingItemsStarted     int64
	maxDownloadWaiting         int
	maxProcessingWaiting       int
	lastBatchSize              int
	lastBatchDuration          time.Duration
	snapshotPersistErrors      int64
	recoveredPanics            int64
	actionAdmissionFailures    int64
	degraded                   bool
	degradedReason             string
	degradedAt                 time.Time
	operations                 telemetry.TimingBook
}

func (m *metricsState) snapshot() MetricsSnapshot {
	if m == nil {
		return MetricsSnapshot{}
	}
	if !m.mu.TryLock() {
		return MetricsSnapshot{}
	}
	snap := MetricsSnapshot{
		DownloadEnqueued:           m.downloadEnqueued,
		DownloadDeferred:           m.downloadDeferred,
		DownloadStarted:            m.downloadStarted,
		DownloadFinished:           m.downloadFinished,
		ProcessingEnqueued:         m.processingEnqueued,
		ProcessingRequeued:         m.processingRequeued,
		ProcessingBatchesStarted:   m.processingBatchesStarted,
		ProcessingBatchesCompleted: m.processingBatchesCompleted,
		ProcessingItemsStarted:     m.processingItemsStarted,
		MaxDownloadWaiting:         m.maxDownloadWaiting,
		MaxProcessingWaiting:       m.maxProcessingWaiting,
		LastBatchSize:              m.lastBatchSize,
		LastBatchDurationMS:        schedulerDurationMillis(m.lastBatchDuration),
		SnapshotPersistErrors:      m.snapshotPersistErrors,
		RecoveredPanics:            m.recoveredPanics,
		ActionAdmissionFailures:    m.actionAdmissionFailures,
		Degraded:                   m.degraded,
		DegradedReason:             m.degradedReason,
		DegradedAt:                 m.degradedAt,
	}
	m.mu.Unlock()
	if operations, ok := m.operations.TrySnapshot(); ok {
		snap.Operations = operations
	}
	return snap
}

func (m *metricsState) recordDownloadEnqueue(waiting int) {
	if m == nil {
		return
	}
	observability.TryCount("scheduler.queue.admissions", 1,
		attribute.String("scheduler.queue", "download"),
		attribute.String("scheduler.result", "queued"))
	observability.TryGauge("scheduler.queue.depth", int64(waiting), attribute.String("scheduler.queue", "download"))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadEnqueued++
	if waiting > m.maxDownloadWaiting {
		m.maxDownloadWaiting = waiting
	}
}

func (m *metricsState) recordDownloadDeferred() {
	if m == nil {
		return
	}
	observability.TryCount("scheduler.queue.admissions", 1,
		attribute.String("scheduler.queue", "download"),
		attribute.String("scheduler.result", "deferred"))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadDeferred++
}

func (m *metricsState) recordDownloadStart() {
	if m == nil {
		return
	}
	observability.TryCount("scheduler.work.started", 1, attribute.String("scheduler.queue", "download"))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadStarted++
}

func (m *metricsState) recordDownloadFinish() {
	if m == nil {
		return
	}
	observability.TryCount("scheduler.work.completed", 1, attribute.String("scheduler.queue", "download"))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadFinished++
}

func (m *metricsState) recordProcessingEnqueue(waiting int) {
	if m == nil {
		return
	}
	observability.TryCount("scheduler.queue.admissions", 1,
		attribute.String("scheduler.queue", "processing"),
		attribute.String("scheduler.result", "queued"))
	observability.TryGauge("scheduler.queue.depth", int64(waiting), attribute.String("scheduler.queue", "processing"))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processingEnqueued++
	if waiting > m.maxProcessingWaiting {
		m.maxProcessingWaiting = waiting
	}
}

func (m *metricsState) recordProcessingRequeue(waiting int) {
	if m == nil {
		return
	}
	observability.TryCount("scheduler.queue.admissions", 1,
		attribute.String("scheduler.queue", "processing"),
		attribute.String("scheduler.result", "requeued"))
	observability.TryGauge("scheduler.queue.depth", int64(waiting), attribute.String("scheduler.queue", "processing"))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processingRequeued++
	if waiting > m.maxProcessingWaiting {
		m.maxProcessingWaiting = waiting
	}
}

func (m *metricsState) recordBatchStart(size int) {
	if m == nil {
		return
	}
	observability.TryCount("scheduler.work.started", 1, attribute.String("scheduler.queue", "processing"))
	observability.TryGauge("scheduler.batch.items", int64(size), attribute.String("scheduler.queue", "processing"))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processingBatchesStarted++
	m.processingItemsStarted += int64(size)
	m.lastBatchSize = size
}

func (m *metricsState) recordBatchComplete(size int, dur time.Duration) {
	if m == nil {
		return
	}
	observability.TryCount("scheduler.work.completed", 1, attribute.String("scheduler.queue", "processing"))
	observability.TryDuration("scheduler.batch", dur, attribute.String("scheduler.queue", "processing"))
	observability.TryGauge("scheduler.batch.items", int64(size), attribute.String("scheduler.queue", "processing"))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processingBatchesCompleted++
	m.lastBatchSize = size
	m.lastBatchDuration = dur
}

func (m *metricsState) observeOperation(name string, dur time.Duration) {
	if m == nil {
		return
	}
	observability.TryDuration(name, dur)
	m.operations.TryObserve(name, dur)
}

func (m *metricsState) recordSnapshotPersistError() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshotPersistErrors++
}

func (m *metricsState) recordRecoveredPanic(component string, now time.Time) {
	if m == nil {
		return
	}
	observability.TryCount("scheduler.recovered_panics", 1,
		attribute.String("scheduler.component", component))
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recoveredPanics++
	m.markDegradedLocked("panic:"+component, now)
}

func (m *metricsState) recordActionAdmissionFailure(now time.Time) {
	if m == nil {
		return
	}
	observability.TryCount("scheduler.action.admission_failures", 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.actionAdmissionFailures++
	m.markDegradedLocked("action_admission_failed", now)
}

func (m *metricsState) markDegradedLocked(reason string, now time.Time) {
	m.degraded = true
	m.degradedReason = reason
	m.degradedAt = now
}

func schedulerDurationMillis(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	ms := d.Milliseconds()
	if ms == 0 {
		return 1
	}
	return ms
}
