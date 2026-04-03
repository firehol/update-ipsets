package telemetry

import (
	"sort"
	"sync"
)

type CounterStatSnapshot struct {
	Name  string `json:"name"`
	Count int64  `json:"count,omitempty"`
	Bytes int64  `json:"bytes,omitempty"`
}

type counterStat struct {
	count int64
	bytes int64
}

type CounterBook struct {
	mu    sync.Mutex
	stats map[string]counterStat
}

func (b *CounterBook) Add(name string, count, bytes int64) {
	if b == nil || name == "" {
		return
	}
	if count < 0 {
		count = 0
	}
	if bytes < 0 {
		bytes = 0
	}
	if count == 0 && bytes == 0 {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stats == nil {
		b.stats = make(map[string]counterStat)
	}
	current := b.stats[name]
	current.count += count
	current.bytes += bytes
	b.stats[name] = current
}

func (b *CounterBook) Snapshot() []CounterStatSnapshot {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.stats) == 0 {
		return nil
	}
	out := make([]CounterStatSnapshot, 0, len(b.stats))
	for name, stat := range b.stats {
		out = append(out, CounterStatSnapshot{
			Name:  name,
			Count: stat.count,
			Bytes: stat.bytes,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}
