package iprange

// OperationStats contains plain counters collected by iprange operations.
// Callers decide how to expose these counters through logs, metrics, or traces.
type OperationStats struct {
	BytesRead          int64
	BytesWritten       int64
	LinesRead          int64
	RangesAccepted     int64
	RangesRead         int64
	RangesScanned      int64
	RangesEmitted      int64
	RangesWritten      int64
	Lookups            int64
	BinarySearches     int64
	Comparisons        int64
	Sources            int64
	HostnamesQueued    int64
	HostnamesCompleted int64
	HostnamesResolved  int64
}

func (s *OperationStats) Add(other OperationStats) {
	if s == nil {
		return
	}
	s.BytesRead += other.BytesRead
	s.BytesWritten += other.BytesWritten
	s.LinesRead += other.LinesRead
	s.RangesAccepted += other.RangesAccepted
	s.RangesRead += other.RangesRead
	s.RangesScanned += other.RangesScanned
	s.RangesEmitted += other.RangesEmitted
	s.RangesWritten += other.RangesWritten
	s.Lookups += other.Lookups
	s.BinarySearches += other.BinarySearches
	s.Comparisons += other.Comparisons
	s.Sources += other.Sources
	s.HostnamesQueued += other.HostnamesQueued
	s.HostnamesCompleted += other.HostnamesCompleted
	s.HostnamesResolved += other.HostnamesResolved
}
