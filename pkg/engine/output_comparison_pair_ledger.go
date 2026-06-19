package engine

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	comparisonPairLedgerFormatVersion    = 1
	comparisonPairLedgerAlgorithmVersion = 1
	comparisonPairLedgerFileName         = "comparison-pairs-v1.json"
	comparisonPairLedgerMaxBytes         = 128 << 20
)

type comparisonPairLedgerSnapshot struct {
	entries map[comparisonPairLedgerKey]uint64
}

type comparisonPairLedgerKey struct {
	left             string
	right            string
	leftHash         comparisonPairLedgerHash
	rightHash        comparisonPairLedgerHash
	algorithmVersion int
}

type comparisonPairLedgerHash struct {
	sum   [32]byte
	valid bool
}

type comparisonPairLedgerFile struct {
	Version          int                         `json:"version"`
	AlgorithmVersion int                         `json:"algorithm_version"`
	Entries          []comparisonPairLedgerEntry `json:"entries"`
}

type comparisonPairLedgerEntry struct {
	Left           string `json:"left"`
	Right          string `json:"right"`
	LeftHash       string `json:"left_hash,omitempty"`
	LeftHashValid  bool   `json:"left_hash_valid,omitempty"`
	RightHash      string `json:"right_hash,omitempty"`
	RightHashValid bool   `json:"right_hash_valid,omitempty"`
	Common         uint64 `json:"common"`
}

func newComparisonPairLedgerSnapshot() *comparisonPairLedgerSnapshot {
	return &comparisonPairLedgerSnapshot{entries: make(map[comparisonPairLedgerKey]uint64)}
}

func (s *comparisonPairLedgerSnapshot) lookup(left, right comparisonSetInfo) (uint64, bool) {
	if s == nil || len(s.entries) == 0 {
		return 0, false
	}
	common, ok := s.entries[comparisonPairLedgerKeyForInfos(left, right)]
	return common, ok
}

func (e *Engine) comparisonPairLedgerPath() string {
	if e == nil || e.runtime.CacheDir == "" {
		return ""
	}
	return filepath.Join(e.runtime.CacheDir, comparisonPairLedgerFileName)
}

func (e *Engine) loadComparisonPairLedger() (*comparisonPairLedgerSnapshot, int, int64, error) {
	snapshot := newComparisonPairLedgerSnapshot()
	path := e.comparisonPairLedgerPath()
	if path == "" {
		return snapshot, 0, 0, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, 0, 0, nil
		}
		return snapshot, 0, 0, fmt.Errorf("stat comparison pair ledger: %w", err)
	}
	if !info.Mode().IsRegular() {
		return snapshot, 0, 0, fmt.Errorf("comparison pair ledger is not a regular file")
	}
	if info.Size() > comparisonPairLedgerMaxBytes {
		return snapshot, 0, 0, fmt.Errorf("comparison pair ledger is too large: %d bytes", info.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshot, 0, 0, fmt.Errorf("read comparison pair ledger: %w", err)
	}
	var disk comparisonPairLedgerFile
	if err := json.Unmarshal(data, &disk); err != nil {
		return snapshot, 0, int64(len(data)), fmt.Errorf("decode comparison pair ledger: %w", err)
	}
	if disk.Version != comparisonPairLedgerFormatVersion {
		return snapshot, 0, int64(len(data)), fmt.Errorf("comparison pair ledger version %d does not match %d", disk.Version, comparisonPairLedgerFormatVersion)
	}
	if disk.AlgorithmVersion != comparisonPairLedgerAlgorithmVersion {
		return snapshot, 0, int64(len(data)), fmt.Errorf("comparison pair ledger algorithm version %d does not match %d", disk.AlgorithmVersion, comparisonPairLedgerAlgorithmVersion)
	}
	for idx, entry := range disk.Entries {
		key, err := entry.key()
		if err != nil {
			return snapshot, 0, int64(len(data)), fmt.Errorf("comparison pair ledger entry %d: %w", idx, err)
		}
		snapshot.entries[key] = entry.Common
	}
	return snapshot, len(snapshot.entries), int64(len(data)), nil
}

func (e *Engine) writeComparisonPairLedger(infos []comparisonSetInfo, results []comparisonPairResult) (int, int64, error) {
	path := e.comparisonPairLedgerPath()
	if path == "" {
		return 0, 0, nil
	}
	byKey := make(map[comparisonPairLedgerKey]uint64, len(results))
	for _, result := range results {
		if result.i < 0 || result.i >= len(infos) || result.j < 0 || result.j >= len(infos) {
			continue
		}
		byKey[comparisonPairLedgerKeyForInfos(infos[result.i], infos[result.j])] = result.common
	}
	entries := make([]comparisonPairLedgerEntry, 0, len(byKey))
	for key, common := range byKey {
		entries = append(entries, comparisonPairLedgerEntryFromKey(key, common))
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Left != entries[j].Left {
			return entries[i].Left < entries[j].Left
		}
		if entries[i].Right != entries[j].Right {
			return entries[i].Right < entries[j].Right
		}
		if entries[i].LeftHash != entries[j].LeftHash {
			return entries[i].LeftHash < entries[j].LeftHash
		}
		return entries[i].RightHash < entries[j].RightHash
	})
	disk := comparisonPairLedgerFile{
		Version:          comparisonPairLedgerFormatVersion,
		AlgorithmVersion: comparisonPairLedgerAlgorithmVersion,
		Entries:          entries,
	}
	data, err := json.MarshalIndent(disk, "", "\t")
	if err != nil {
		return 0, 0, err
	}
	data = append(data, '\n')
	if err := writeFileAtomic(path, data, generatedFileMode); err != nil {
		return 0, int64(len(data)), err
	}
	return len(entries), int64(len(data)), nil
}

func comparisonPairLedgerKeyForInfos(left, right comparisonSetInfo) comparisonPairLedgerKey {
	leftHash := comparisonPairLedgerHash{
		sum:   left.contentHash.sum,
		valid: left.contentHash.valid,
	}
	rightHash := comparisonPairLedgerHash{
		sum:   right.contentHash.sum,
		valid: right.contentHash.valid,
	}
	leftName, rightName := left.name, right.name
	if rightName < leftName {
		leftName, rightName = rightName, leftName
		leftHash, rightHash = rightHash, leftHash
	}
	return comparisonPairLedgerKey{
		left:             leftName,
		right:            rightName,
		leftHash:         leftHash,
		rightHash:        rightHash,
		algorithmVersion: comparisonPairLedgerAlgorithmVersion,
	}
}

func comparisonPairLedgerEntryFromKey(key comparisonPairLedgerKey, common uint64) comparisonPairLedgerEntry {
	return comparisonPairLedgerEntry{
		Left:           key.left,
		Right:          key.right,
		LeftHash:       encodeComparisonPairLedgerHash(key.leftHash),
		LeftHashValid:  key.leftHash.valid,
		RightHash:      encodeComparisonPairLedgerHash(key.rightHash),
		RightHashValid: key.rightHash.valid,
		Common:         common,
	}
}

func (e comparisonPairLedgerEntry) key() (comparisonPairLedgerKey, error) {
	if e.Left == "" {
		return comparisonPairLedgerKey{}, fmt.Errorf("left feed name is empty")
	}
	if e.Right == "" {
		return comparisonPairLedgerKey{}, fmt.Errorf("right feed name is empty")
	}
	leftHash, err := decodeComparisonPairLedgerHash(e.LeftHash, e.LeftHashValid)
	if err != nil {
		return comparisonPairLedgerKey{}, fmt.Errorf("left hash: %w", err)
	}
	rightHash, err := decodeComparisonPairLedgerHash(e.RightHash, e.RightHashValid)
	if err != nil {
		return comparisonPairLedgerKey{}, fmt.Errorf("right hash: %w", err)
	}
	key := comparisonPairLedgerKey{
		left:             e.Left,
		right:            e.Right,
		leftHash:         leftHash,
		rightHash:        rightHash,
		algorithmVersion: comparisonPairLedgerAlgorithmVersion,
	}
	if key.right < key.left {
		key.left, key.right = key.right, key.left
		key.leftHash, key.rightHash = key.rightHash, key.leftHash
	}
	return key, nil
}

func encodeComparisonPairLedgerHash(hash comparisonPairLedgerHash) string {
	if !hash.valid {
		return ""
	}
	return hex.EncodeToString(hash.sum[:])
}

func decodeComparisonPairLedgerHash(value string, valid bool) (comparisonPairLedgerHash, error) {
	if !valid {
		if value != "" {
			return comparisonPairLedgerHash{}, fmt.Errorf("invalid hash marker has non-empty hash")
		}
		return comparisonPairLedgerHash{}, nil
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return comparisonPairLedgerHash{}, err
	}
	if len(decoded) != 32 {
		return comparisonPairLedgerHash{}, fmt.Errorf("decoded length %d is not 32", len(decoded))
	}
	var hash comparisonPairLedgerHash
	copy(hash.sum[:], decoded)
	hash.valid = true
	return hash, nil
}
