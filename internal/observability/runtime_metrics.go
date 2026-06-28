package observability

import (
	"context"
	"sync"
	"time"

	"github.com/firehol/update-ipsets/internal/runtimeinfo"
)

const runtimeMetricsSampleInterval = time.Second

type runtimeMetricSampler struct {
	stop    sync.Once
	stopC   chan struct{}
	stopped chan struct{}
}

func startRuntimeMetricSampler(interval time.Duration) func(context.Context) error {
	if interval <= 0 {
		interval = runtimeMetricsSampleInterval
	}
	observeRuntimeMetrics()
	sampler := &runtimeMetricSampler{
		stopC:   make(chan struct{}),
		stopped: make(chan struct{}),
	}
	go sampler.run(interval)
	return sampler.shutdown
}

func (s *runtimeMetricSampler) run(interval time.Duration) {
	defer close(s.stopped)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			observeRuntimeMetrics()
		case <-s.stopC:
			return
		}
	}
}

func (s *runtimeMetricSampler) shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.stop.Do(func() { close(s.stopC) })
	select {
	case <-s.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func observeRuntimeMetrics() {
	snap := runtimeinfo.Capture()
	TryGauge("runtime.go.goroutines", int64(snap.Goroutines))
	TryGauge("runtime.go.heap.alloc.bytes", uint64MetricValue(snap.HeapAlloc))
	TryGauge("runtime.go.heap.sys.bytes", uint64MetricValue(snap.HeapSys))
	TryGauge("runtime.go.heap.inuse.bytes", uint64MetricValue(snap.HeapInuse))
	TryGauge("runtime.go.heap.released.bytes", uint64MetricValue(snap.HeapReleased))
	TryGauge("runtime.go.heap.objects", uint64MetricValue(snap.HeapObjects))
	TryGauge("runtime.go.stack.inuse.bytes", uint64MetricValue(snap.StackInuse))
	TryGauge("runtime.go.sys.bytes", uint64MetricValue(snap.Sys))
	TryGauge("runtime.go.gc.count", int64(snap.NumGC))
	TryGauge("runtime.go.gc.pause.total.ms", snap.PauseTotalMS)
	TryGauge("runtime.go.mem.limit.bytes", snap.GoMemLimit)
	TryGauge("runtime.process.rss.bytes", kibMetricValue(snap.RSSKB))
	TryGauge("runtime.process.vms.bytes", kibMetricValue(snap.VMSKB))
	TryGauge("runtime.process.data.bytes", kibMetricValue(snap.DataKB))
	TryGauge("runtime.process.cpu.user.ms", snap.CPUUserMS)
	TryGauge("runtime.process.cpu.system.ms", snap.CPUSystemMS)
	TryGauge("runtime.process.cpu.total.ms", snap.CPUTotalMS)
	TryGauge("runtime.process.read.bytes", uint64MetricValue(snap.ProcReadBytes))
	TryGauge("runtime.process.write.bytes", uint64MetricValue(snap.ProcWriteBytes))
	TryGauge("runtime.process.cancelled_write.bytes", uint64MetricValue(snap.ProcCancelledWriteBytes))
	TryGauge("runtime.process.read.syscalls", uint64MetricValue(snap.ProcReadSyscalls))
	TryGauge("runtime.process.write.syscalls", uint64MetricValue(snap.ProcWriteSyscalls))
	TryGauge("runtime.process.open_fds", int64(snap.OpenFDs))
}

func kibMetricValue(value uint64) int64 {
	const maxKiB = uint64MetricMax / 1024
	if value > maxKiB {
		return int64(uint64MetricMax)
	}
	return int64(value * 1024)
}

func uint64MetricValue(value uint64) int64 {
	if value > uint64MetricMax {
		return int64(uint64MetricMax)
	}
	return int64(value)
}

const uint64MetricMax = uint64(^uint64(0) >> 1)
