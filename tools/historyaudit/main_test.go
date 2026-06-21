package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildManifestReportsHistoryRetentionArtifacts(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	feedDir := filepath.Join(libDir, "sample")
	newDir := filepath.Join(feedDir, "new")
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		t.Fatal(err)
	}

	historyBody := "DateTime,Entries,UniqueIPs\n1700000000,10,100\n1700003600,20,200\n"
	writeTestFile(t, filepath.Join(feedDir, "history.csv"), historyBody)
	writeTestFile(t, filepath.Join(feedDir, "retention.csv"), "date_removed,date_added,hours,ips\n1700007200,1700000000,2,5\n")
	writeTestFile(t, filepath.Join(feedDir, "retention.json"), "{\"past\":{\"total\":5}}\n")
	writeTestFile(t, filepath.Join(feedDir, "retention_cohorts.csv"), "date_added,ips\n1700000000,95\n")
	writeTestFile(t, filepath.Join(newDir, "1700000000"), "binary-ish cohort bytes")
	writeTestFile(t, filepath.Join(newDir, ".tmp-ignored"), "partial")
	writeTestFile(t, filepath.Join(libDir, "asn", "provider-state"), "not a feed history artifact")

	manifest, err := BuildManifest(libDir)
	if err != nil {
		t.Fatalf("BuildManifest() error = %v", err)
	}
	if got, want := manifest.Version, manifestVersion; got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
	if got, want := manifest.Totals.Feeds, 1; got != want {
		t.Fatalf("feeds = %d, want %d", got, want)
	}
	if got, want := manifest.Totals.Artifacts, 5; got != want {
		t.Fatalf("artifacts = %d, want %d", got, want)
	}
	if got, want := manifest.Totals.HistoryRows, int64(2); got != want {
		t.Fatalf("history rows = %d, want %d", got, want)
	}
	if got, want := manifest.Totals.RetentionRows, int64(1); got != want {
		t.Fatalf("retention rows = %d, want %d", got, want)
	}
	if got, want := manifest.Totals.RetentionCohorts, int64(1); got != want {
		t.Fatalf("retention cohorts = %d, want %d", got, want)
	}

	feed := manifest.Feeds[0]
	if got, want := feed.Name, "sample"; got != want {
		t.Fatalf("feed name = %q, want %q", got, want)
	}
	history := findArtifact(t, feed, "history_csv")
	if got, want := history.Path, "sample/history.csv"; got != want {
		t.Fatalf("history path = %q, want %q", got, want)
	}
	if got, want := history.SHA256, sha256Hex(historyBody); got != want {
		t.Fatalf("history sha256 = %s, want %s", got, want)
	}
	if got, want := history.FirstTimestamp, int64(1700000000); got != want {
		t.Fatalf("first timestamp = %d, want %d", got, want)
	}
	if got, want := history.LastTimestamp, int64(1700003600); got != want {
		t.Fatalf("last timestamp = %d, want %d", got, want)
	}
	if !history.MonotonicTimestamps {
		t.Fatal("history timestamps are not monotonic")
	}
	if findArtifactByPath(feed, "sample/new/.tmp-ignored") != nil {
		t.Fatal("hidden atomic temp file was included in manifest")
	}
}

func TestBuildManifestFlagsNonMonotonicHistory(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "lib")
	feedDir := filepath.Join(libDir, "sample")
	if err := os.MkdirAll(feedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(feedDir, "history.csv"), "DateTime,Entries,UniqueIPs\n1700003600,20,200\n1700000000,10,100\n")

	manifest, err := BuildManifest(libDir)
	if err != nil {
		t.Fatalf("BuildManifest() error = %v", err)
	}
	history := findArtifact(t, manifest.Feeds[0], "history_csv")
	if !history.TimestampChecked {
		t.Fatal("history timestamp check was not recorded")
	}
	if history.MonotonicTimestamps {
		t.Fatal("non-monotonic history timestamps were not flagged")
	}
	if got, want := history.Rows, int64(2); got != want {
		t.Fatalf("history rows = %d, want %d", got, want)
	}
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func findArtifact(t *testing.T, feed FeedManifest, kind string) ArtifactRecord {
	t.Helper()
	for _, artifact := range feed.Artifacts {
		if artifact.Kind == kind {
			return artifact
		}
	}
	t.Fatalf("missing artifact kind %q in %+v", kind, feed.Artifacts)
	return ArtifactRecord{}
}

func findArtifactByPath(feed FeedManifest, path string) *ArtifactRecord {
	for i := range feed.Artifacts {
		if feed.Artifacts[i].Path == path {
			return &feed.Artifacts[i]
		}
	}
	return nil
}

func sha256Hex(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}
