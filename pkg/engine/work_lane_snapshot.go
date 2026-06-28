package engine

import (
	"slices"
	"time"
)

func (l *WorkLane) TrySnapshot() (LaneSnapshot, bool) {
	return l.trySnapshotAt(time.Now().UTC())
}

func (l *WorkLane) snapshotAt(now time.Time) LaneSnapshot {
	if l == nil {
		return LaneSnapshot{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.snapshotLocked(now)
}

func (l *WorkLane) trySnapshotAt(now time.Time) (LaneSnapshot, bool) {
	if l == nil {
		return LaneSnapshot{}, true
	}
	if !l.mu.TryLock() {
		return LaneSnapshot{}, false
	}
	defer l.mu.Unlock()
	return l.snapshotLocked(now), true
}

func (l *WorkLane) snapshotLocked(now time.Time) LaneSnapshot {
	active := make([]LaneWorkSnapshot, 0, len(l.active))
	for _, item := range l.active {
		active = append(active, item.snapshot(now))
	}
	slices.SortFunc(active, func(a, b LaneWorkSnapshot) int {
		return a.QueuedAt.Compare(b.QueuedAt)
	})
	waiting := make([]LaneWorkSnapshot, 0, len(l.queue))
	for _, item := range l.queue {
		if item.state == LaneWorkQueued {
			waiting = append(waiting, item.snapshot(now))
		}
	}
	return LaneSnapshot{
		Limit:        l.limit,
		ActiveCount:  len(active),
		WaitingCount: len(waiting),
		Active:       active,
		Waiting:      waiting,
	}
}
