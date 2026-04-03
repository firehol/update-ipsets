package telemetry

import (
	"sort"
	"sync"
	"time"
)

type TimingStatSnapshot struct {
	Name    string `json:"name"`
	Count   int64  `json:"count"`
	TotalMS int64  `json:"total_ms"`
	AvgMS   int64  `json:"avg_ms"`
	MaxMS   int64  `json:"max_ms"`
}

type timingStat struct {
	count int64
	total time.Duration
	max   time.Duration
}

type TimingBook struct {
	mu    sync.Mutex
	stats map[string]timingStat
}

func (b *TimingBook) Observe(name string, dur time.Duration) {
	b.ObserveAggregate(name, 1, dur, dur)
}

func (b *TimingBook) ObserveAggregate(name string, count int64, total, max time.Duration) {
	if b == nil || name == "" || count <= 0 {
		return
	}
	if total < 0 {
		total = 0
	}
	if max < 0 {
		max = 0
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stats == nil {
		b.stats = make(map[string]timingStat)
	}
	current := b.stats[name]
	current.count += count
	current.total += total
	if max > current.max {
		current.max = max
	}
	b.stats[name] = current
}

func (b *TimingBook) Snapshot() []TimingStatSnapshot {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.stats) == 0 {
		return nil
	}
	out := make([]TimingStatSnapshot, 0, len(b.stats))
	for name, stat := range b.stats {
		avg := time.Duration(0)
		if stat.count > 0 {
			avg = stat.total / time.Duration(stat.count)
		}
		out = append(out, TimingStatSnapshot{
			Name:    name,
			Count:   stat.count,
			TotalMS: durationMillis(stat.total),
			AvgMS:   durationMillis(avg),
			MaxMS:   durationMillis(stat.max),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalMS != out[j].TotalMS {
			return out[i].TotalMS > out[j].TotalMS
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func durationMillis(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	ms := d.Milliseconds()
	if ms == 0 {
		return 1
	}
	return ms
}
