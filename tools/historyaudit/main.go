package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const manifestVersion = 1

type Manifest struct {
	Version int            `json:"version"`
	LibDir  string         `json:"lib_dir,omitempty"`
	Totals  ManifestTotals `json:"totals"`
	Feeds   []FeedManifest `json:"feeds"`
}

type ManifestTotals struct {
	Feeds            int   `json:"feeds"`
	Artifacts        int   `json:"artifacts"`
	Bytes            int64 `json:"bytes"`
	HistoryRows      int64 `json:"history_rows"`
	RetentionRows    int64 `json:"retention_rows"`
	RetentionCohorts int64 `json:"retention_cohorts"`
}

type FeedManifest struct {
	Name      string           `json:"name"`
	Totals    ManifestTotals   `json:"totals"`
	Artifacts []ArtifactRecord `json:"artifacts"`
}

type ArtifactRecord struct {
	Kind                string `json:"kind"`
	Path                string `json:"path"`
	SizeBytes           int64  `json:"size_bytes"`
	ModTimeUnix         int64  `json:"mod_time_unix"`
	SHA256              string `json:"sha256"`
	TimestampChecked    bool   `json:"timestamp_checked,omitempty"`
	Rows                int64  `json:"rows,omitempty"`
	MalformedRows       int64  `json:"malformed_rows,omitempty"`
	FirstTimestamp      int64  `json:"first_timestamp,omitempty"`
	LastTimestamp       int64  `json:"last_timestamp,omitempty"`
	MonotonicTimestamps bool   `json:"monotonic_timestamps,omitempty"`
}

func main() {
	var libDir string
	var includeRoot bool
	flag.StringVar(&libDir, "lib-dir", "", "path to the update-ipsets lib directory")
	flag.BoolVar(&includeRoot, "include-root", false, "include the absolute lib directory path in the manifest")
	flag.Parse()
	if libDir == "" && flag.NArg() > 0 {
		libDir = flag.Arg(0)
	}
	if libDir == "" {
		fmt.Fprintln(os.Stderr, "usage: historyaudit -lib-dir /path/to/lib")
		os.Exit(2)
	}

	manifest, err := BuildManifest(libDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "historyaudit: %v\n", err)
		os.Exit(1)
	}
	if includeRoot {
		abs, err := filepath.Abs(libDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "historyaudit: %v\n", err)
			os.Exit(1)
		}
		manifest.LibDir = abs
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		fmt.Fprintf(os.Stderr, "historyaudit: encode manifest: %v\n", err)
		os.Exit(1)
	}
}

func BuildManifest(libDir string) (Manifest, error) {
	entries, err := os.ReadDir(libDir)
	if err != nil {
		return Manifest{}, fmt.Errorf("read lib dir: %w", err)
	}
	manifest := Manifest{Version: manifestVersion}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		feed, err := scanFeed(libDir, entry.Name())
		if err != nil {
			return Manifest{}, err
		}
		if len(feed.Artifacts) == 0 {
			continue
		}
		manifest.Feeds = append(manifest.Feeds, feed)
		addTotals(&manifest.Totals, feed.Totals)
	}
	sort.Slice(manifest.Feeds, func(i, j int) bool {
		return manifest.Feeds[i].Name < manifest.Feeds[j].Name
	})
	manifest.Totals.Feeds = len(manifest.Feeds)
	return manifest, nil
}

func scanFeed(libDir, name string) (FeedManifest, error) {
	feed := FeedManifest{Name: name}
	dir := filepath.Join(libDir, name)
	for _, spec := range []struct {
		kind   string
		rel    string
		tsCol  int
		hasCSV bool
	}{
		{kind: "history_csv", rel: "history.csv", tsCol: 0, hasCSV: true},
		{kind: "retention_csv", rel: "retention.csv", tsCol: 0, hasCSV: true},
		{kind: "retention_json", rel: "retention.json"},
		{kind: "retention_cohorts_csv", rel: "retention_cohorts.csv", tsCol: 0, hasCSV: true},
	} {
		path := filepath.Join(dir, spec.rel)
		exists, err := pathExists(path)
		if err != nil {
			return FeedManifest{}, fmt.Errorf("stat %s: %w", filepath.Join(name, spec.rel), err)
		}
		if !exists {
			continue
		}
		record, err := scanArtifact(libDir, filepath.Join(name, spec.rel), spec.kind)
		if err != nil {
			return FeedManifest{}, err
		}
		if spec.hasCSV {
			if err := inspectCSV(path, spec.tsCol, &record); err != nil {
				return FeedManifest{}, err
			}
		}
		feed.Artifacts = append(feed.Artifacts, record)
		addArtifactTotals(&feed.Totals, record)
	}

	newDir := filepath.Join(dir, "new")
	entries, err := os.ReadDir(newDir)
	if err != nil && !os.IsNotExist(err) {
		return FeedManifest{}, fmt.Errorf("read retention cohort dir %s: %w", filepath.Join(name, "new"), err)
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		rel := filepath.Join(name, "new", entry.Name())
		record, err := scanArtifact(libDir, rel, "retention_cohort")
		if err != nil {
			return FeedManifest{}, err
		}
		feed.Artifacts = append(feed.Artifacts, record)
		addArtifactTotals(&feed.Totals, record)
	}

	sort.Slice(feed.Artifacts, func(i, j int) bool {
		return feed.Artifacts[i].Path < feed.Artifacts[j].Path
	})
	feed.Totals.Feeds = 1
	return feed, nil
}

func scanArtifact(rootDir, rel, kind string) (ArtifactRecord, error) {
	path := filepath.Join(rootDir, rel)
	info, err := os.Stat(path)
	if err != nil {
		return ArtifactRecord{}, fmt.Errorf("stat %s: %w", rel, err)
	}
	sum, err := fileSHA256(path, sha256.New())
	if err != nil {
		return ArtifactRecord{}, fmt.Errorf("checksum %s: %w", rel, err)
	}
	return ArtifactRecord{
		Kind:        kind,
		Path:        filepath.ToSlash(rel),
		SizeBytes:   info.Size(),
		ModTimeUnix: info.ModTime().Unix(),
		SHA256:      sum,
	}, nil
}

func fileSHA256(path string, h hash.Hash) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	if _, err := io.Copy(h, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func inspectCSV(path string, timestampColumn int, record *ArtifactRecord) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	firstLine := true
	var previous int64
	record.TimestampChecked = true
	record.MonotonicTimestamps = true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if firstLine {
			firstLine = false
			continue
		}
		record.Rows++
		parts := strings.Split(line, ",")
		if timestampColumn >= len(parts) {
			record.MalformedRows++
			record.MonotonicTimestamps = false
			continue
		}
		ts, err := strconv.ParseInt(strings.TrimSpace(parts[timestampColumn]), 10, 64)
		if err != nil {
			record.MalformedRows++
			record.MonotonicTimestamps = false
			continue
		}
		if record.Rows == 1 {
			record.FirstTimestamp = ts
		} else if ts < previous {
			record.MonotonicTimestamps = false
		}
		previous = ts
		record.LastTimestamp = ts
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func addArtifactTotals(totals *ManifestTotals, record ArtifactRecord) {
	totals.Artifacts++
	totals.Bytes += record.SizeBytes
	switch record.Kind {
	case "history_csv":
		totals.HistoryRows += record.Rows
	case "retention_csv":
		totals.RetentionRows += record.Rows
	case "retention_cohort":
		totals.RetentionCohorts++
	}
}

func addTotals(dst *ManifestTotals, src ManifestTotals) {
	dst.Artifacts += src.Artifacts
	dst.Bytes += src.Bytes
	dst.HistoryRows += src.HistoryRows
	dst.RetentionRows += src.RetentionRows
	dst.RetentionCohorts += src.RetentionCohorts
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
