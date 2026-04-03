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
	operations                 telemetry.TimingBook
}

func (m *metricsState) snapshot() MetricsSnapshot {
	if m == nil {
		return MetricsSnapshot{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return MetricsSnapshot{
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
		Operations:                 m.operations.Snapshot(),
	}
}

func (m *metricsState) recordDownloadEnqueue(waiting int) {
	if m == nil {
		return
	}
	observability.Count(observability.BackgroundContext(), "download.queued", 1, attribute.Int("scheduler.waiting", waiting))
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
	observability.Count(observability.BackgroundContext(), "download.deferred", 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadDeferred++
}

func (m *metricsState) recordDownloadStart() {
	if m == nil {
		return
	}
	observability.Count(observability.BackgroundContext(), "download.started", 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadStarted++
}

func (m *metricsState) recordDownloadFinish() {
	if m == nil {
		return
	}
	observability.Count(observability.BackgroundContext(), "download.finished", 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.downloadFinished++
}

func (m *metricsState) recordProcessingEnqueue(waiting int) {
	if m == nil {
		return
	}
	observability.Count(observability.BackgroundContext(), "engine.queued", 1, attribute.Int("scheduler.waiting", waiting))
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
	observability.Count(observability.BackgroundContext(), "engine.requeued", 1, attribute.Int("scheduler.waiting", waiting))
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
	observability.Count(observability.BackgroundContext(), "engine.batch.started", 1, attribute.Int("engine.batch.size", size))
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
	observability.Observe(observability.BackgroundContext(), "engine.batch.completed", 1, 0, dur, attribute.Int("engine.batch.size", size))
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
	observability.Duration(observability.BackgroundContext(), name, dur)
	m.operations.Observe(name, dur)
}

func (m *metricsState) recordSnapshotPersistError() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshotPersistErrors++
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
