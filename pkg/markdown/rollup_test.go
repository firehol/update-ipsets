package markdown_test

import (
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/markdown"
)

func TestRollup(t *testing.T) {
	t.Parallel()

	t.Run("empty input returns empty", func(t *testing.T) {
		t.Parallel()
		got := markdown.Rollup(nil, nil)
		if len(got.Rows) != 0 {
			t.Fatalf("expected empty rows")
		}
	})

	t.Run("raw rows for short change history", func(t *testing.T) {
		t.Parallel()

		base := time.Now().Add(-5 * time.Hour)
		history := make([]markdown.HistoryPoint, 6)
		for i := range history {
			history[i] = markdown.HistoryPoint{
				Timestamp: base.Add(time.Duration(i) * time.Hour).Unix(),
				Entries:   100 + i,
				IPs:       uint64(1000 + i*10),
			}
		}

		changesets := []markdown.ChangesetPoint{
			{Timestamp: history[1].Timestamp, Added: 20, Removed: 10},
			{Timestamp: history[3].Timestamp, Added: 15, Removed: 5},
		}

		got := markdown.BuildActivity(history, changesets, 0)
		if len(got.Rows) == 0 {
			t.Fatal("expected non-empty rows")
		}
		if got.Resolution != "observed update" {
			t.Fatalf("Resolution=%q; want observed update for short change history", got.Resolution)
		}
		if got.LatestIPs != history[len(history)-1].IPs {
			t.Fatalf("LatestIPs=%d; want %d", got.LatestIPs, history[len(history)-1].IPs)
		}
	})

	t.Run("daily rollup for longer active history", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		history := make([]markdown.HistoryPoint, 200)
		changesets := make([]markdown.ChangesetPoint, 0, len(history))
		for i := range history {
			ts := now.Add(-time.Duration(200-i) * time.Hour).Unix()
			history[i] = markdown.HistoryPoint{
				Timestamp: ts,
				Entries:   100,
				IPs:       uint64(1000 + i),
			}
			changesets = append(changesets, markdown.ChangesetPoint{
				Timestamp: ts,
				Added:     1,
				Removed:   1,
			})
		}

		got := markdown.BuildActivity(history, changesets, 0)
		if got.Resolution != "1d" {
			t.Fatalf("Resolution=%q; want '1d' for 200-hour history", got.Resolution)
		}
		if len(got.Rows) > 100 {
			t.Fatalf("len(Rows)=%d; want <=100 after rollup", len(got.Rows))
		}
	})

	t.Run("rollup is not smaller than configured cadence", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		history := make([]markdown.HistoryPoint, 200)
		changesets := make([]markdown.ChangesetPoint, 0, len(history))
		for i := range history {
			ts := now.Add(-time.Duration(200-i) * time.Hour).Unix()
			history[i] = markdown.HistoryPoint{
				Timestamp: ts,
				Entries:   100,
				IPs:       uint64(1000 + i),
			}
			changesets = append(changesets, markdown.ChangesetPoint{
				Timestamp: ts,
				Added:     1,
				Removed:   0,
			})
		}

		got := markdown.BuildActivity(history, changesets, 1440)
		if got.Resolution != "1d" {
			t.Fatalf("Resolution=%q; want '1d' for daily configured cadence", got.Resolution)
		}
		if !got.Rollup {
			t.Fatal("expected rolled-up activity")
		}
	})

	t.Run("churn percentage computed", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		history := []markdown.HistoryPoint{
			{Timestamp: now.Add(-2 * time.Hour).Unix(), Entries: 100, IPs: 1000},
			{Timestamp: now.Add(-1 * time.Hour).Unix(), Entries: 100, IPs: 1020},
		}
		changesets := []markdown.ChangesetPoint{
			{Timestamp: history[1].Timestamp, Added: 30, Removed: 10},
		}

		got := markdown.BuildActivity(history, changesets, 0)
		if len(got.Rows) != 1 {
			t.Fatalf("len(Rows)=%d; want only active change row", len(got.Rows))
		}

		lastRow := got.Rows[len(got.Rows)-1]
		if lastRow.ChurnPct <= 0 {
			t.Fatalf("ChurnPct=%.2f; want >0", lastRow.ChurnPct)
		}
	})

	t.Run("sparse hourly history omits inactive rows", func(t *testing.T) {
		t.Parallel()

		base := time.Now().Add(-5 * time.Hour)
		history := make([]markdown.HistoryPoint, 6)
		for i := range history {
			history[i] = markdown.HistoryPoint{
				Timestamp: base.Add(time.Duration(i) * time.Hour).Unix(),
				Entries:   100,
				IPs:       uint64(1000 + i),
			}
		}

		changesets := []markdown.ChangesetPoint{
			{Timestamp: history[4].Timestamp, Added: 7, Removed: 3},
		}

		got := markdown.BuildActivity(history, changesets, 0)
		if got.Resolution != "observed update" {
			t.Fatalf("Resolution=%q; want observed update", got.Resolution)
		}
		if len(got.Rows) != 1 {
			t.Fatalf("len(Rows)=%d; want only the active bucket", len(got.Rows))
		}
		if got.Rows[0].Added != 7 || got.Rows[0].Removed != 3 {
			t.Fatalf("active row counters = added %.0f removed %.0f; want 7/3", got.Rows[0].Added, got.Rows[0].Removed)
		}
	})

	t.Run("history without changes produces no activity rows", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		got := markdown.BuildActivity([]markdown.HistoryPoint{
			{Timestamp: now.Add(-2 * time.Hour).Unix(), Entries: 100, IPs: 1000},
			{Timestamp: now.Add(-1 * time.Hour).Unix(), Entries: 101, IPs: 1001},
		}, nil, 0)

		if len(got.Rows) != 0 {
			t.Fatalf("len(Rows)=%d; want no rows without observed changes", len(got.Rows))
		}
	})
}

func TestCadence(t *testing.T) {
	t.Parallel()

	t.Run("bins inter-update intervals", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		changesets := []markdown.ChangesetPoint{
			{Timestamp: now.Add(-6 * time.Hour).Unix(), Added: 10, Removed: 0},
			{Timestamp: now.Add(-5 * time.Hour).Unix(), Added: 5, Removed: 2},
			{Timestamp: now.Add(-2 * time.Hour).Unix(), Added: 8, Removed: 3},
			{Timestamp: now.Add(-1 * time.Hour).Unix(), Added: 3, Removed: 1},
		}

		got := markdown.Rollup([]markdown.HistoryPoint{
			{Timestamp: now.Add(-6 * time.Hour).Unix(), IPs: 1000},
			{Timestamp: now.Add(-5 * time.Hour).Unix(), IPs: 1003},
			{Timestamp: now.Add(-2 * time.Hour).Unix(), IPs: 1008},
			{Timestamp: now.Add(-1 * time.Hour).Unix(), IPs: 1010},
		}, changesets)

		if len(got.Cadence) == 0 {
			t.Fatal("expected non-empty cadence bins")
		}

		total := 0
		for _, b := range got.Cadence {
			total += b.Count
		}
		if total != 3 {
			t.Fatalf("total cadence count=%d; want 3 gaps from 4 points", total)
		}
	})

	t.Run("fewer than 2 history points yields no cadence", func(t *testing.T) {
		t.Parallel()

		changesets := []markdown.ChangesetPoint{
			{Timestamp: time.Now().Unix(), Added: 10, Removed: 0},
		}

		got := markdown.Rollup([]markdown.HistoryPoint{
			{Timestamp: time.Now().Unix(), IPs: 1000},
		}, changesets)

		if len(got.Cadence) != 0 {
			t.Fatalf("expected no cadence for <2 changesets")
		}
	})
}
