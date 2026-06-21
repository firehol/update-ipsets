package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	comparisonPairLedgerFormatVersion    = 2
	comparisonPairLedgerAlgorithmVersion = 1
	comparisonPairLedgerFileName         = "comparison-pairs-v2.bin"
	comparisonPairLegacyLedgerFileName   = "comparison-pairs-v1.json"
	comparisonPairLedgerMaxBytes         = 128 << 20
	comparisonPairLedgerHeaderBytes      = 24
	comparisonPairLedgerEntryFixedBytes  = 81
	comparisonPairLedgerMaxNameBytes     = 1<<16 - 1
)

var (
	comparisonPairLedgerMagic       = [8]byte{'U', 'I', 'P', 'C', 'P', 'L', 'D', 'G'}
	errComparisonPairLedgerMissing  = errors.New("comparison pair ledger missing")
	errComparisonPairLedgerTooShort = errors.New("comparison pair ledger is truncated")
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
	Version          int
	AlgorithmVersion int
	Entries          []comparisonPairLedgerEntry
}

type comparisonPairLedgerEntry struct {
	Left           string
	Right          string
	LeftHash       [32]byte
	LeftHashValid  bool
	RightHash      [32]byte
	RightHashValid bool
	Common         uint64
}

type comparisonPairLegacyLedgerFile struct {
	Version          int                               `json:"version"`
	AlgorithmVersion int                               `json:"algorithm_version"`
	Entries          []comparisonPairLegacyLedgerEntry `json:"entries"`
}

type comparisonPairLegacyLedgerEntry struct {
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

func (e *Engine) comparisonPairLegacyLedgerPath() string {
	if e == nil || e.runtime.CacheDir == "" {
		return ""
	}
	return filepath.Join(e.runtime.CacheDir, comparisonPairLegacyLedgerFileName)
}

func (e *Engine) loadComparisonPairLedger() (*comparisonPairLedgerSnapshot, int, int64, error) {
	snapshot := newComparisonPairLedgerSnapshot()
	path := e.comparisonPairLedgerPath()
	if path == "" {
		return snapshot, 0, 0, errComparisonPairLedgerMissing
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return e.loadComparisonPairLegacyLedger()
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
	disk, err := parseComparisonPairLedgerFile(data)
	if err != nil {
		return snapshot, 0, int64(len(data)), fmt.Errorf("decode comparison pair ledger: %w", err)
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

func (e *Engine) loadComparisonPairLegacyLedger() (*comparisonPairLedgerSnapshot, int, int64, error) {
	snapshot := newComparisonPairLedgerSnapshot()
	path := e.comparisonPairLegacyLedgerPath()
	if path == "" {
		return snapshot, 0, 0, errComparisonPairLedgerMissing
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, 0, 0, errComparisonPairLedgerMissing
		}
		return snapshot, 0, 0, fmt.Errorf("stat legacy comparison pair ledger: %w", err)
	}
	if !info.Mode().IsRegular() {
		return snapshot, 0, 0, fmt.Errorf("legacy comparison pair ledger is not a regular file")
	}
	if info.Size() > comparisonPairLedgerMaxBytes {
		return snapshot, 0, 0, fmt.Errorf("legacy comparison pair ledger is too large: %d bytes", info.Size())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshot, 0, 0, fmt.Errorf("read legacy comparison pair ledger: %w", err)
	}
	var disk comparisonPairLegacyLedgerFile
	if err := json.Unmarshal(data, &disk); err != nil {
		return snapshot, 0, int64(len(data)), fmt.Errorf("decode legacy comparison pair ledger: %w", err)
	}
	if disk.Version != 1 {
		return snapshot, 0, int64(len(data)), fmt.Errorf("legacy comparison pair ledger version %d does not match 1", disk.Version)
	}
	if disk.AlgorithmVersion != comparisonPairLedgerAlgorithmVersion {
		return snapshot, 0, int64(len(data)), fmt.Errorf("legacy comparison pair ledger algorithm version %d does not match %d", disk.AlgorithmVersion, comparisonPairLedgerAlgorithmVersion)
	}
	for idx, entry := range disk.Entries {
		key, err := entry.key()
		if err != nil {
			return snapshot, 0, int64(len(data)), fmt.Errorf("legacy comparison pair ledger entry %d: %w", idx, err)
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
		if cmp := bytes.Compare(entries[i].LeftHash[:], entries[j].LeftHash[:]); cmp != 0 {
			return cmp < 0
		}
		return bytes.Compare(entries[i].RightHash[:], entries[j].RightHash[:]) < 0
	})
	disk := comparisonPairLedgerFile{
		Version:          comparisonPairLedgerFormatVersion,
		AlgorithmVersion: comparisonPairLedgerAlgorithmVersion,
		Entries:          entries,
	}
	data, err := marshalComparisonPairLedgerFile(disk)
	if err != nil {
		return 0, 0, err
	}
	if err := writeFileAtomic(path, data, generatedFileMode); err != nil {
		return 0, int64(len(data)), err
	}
	if err := e.removeComparisonPairLegacyLedger(); err != nil {
		return len(entries), int64(len(data)), err
	}
	return len(entries), int64(len(data)), nil
}

func (e *Engine) removeComparisonPairLegacyLedger() error {
	path := e.comparisonPairLegacyLedgerPath()
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy comparison pair ledger: %w", err)
	}
	return nil
}

func marshalComparisonPairLedgerFile(disk comparisonPairLedgerFile) ([]byte, error) {
	names := make([]string, 0)
	nameSet := make(map[string]struct{})
	for _, entry := range disk.Entries {
		if entry.Left == "" {
			return nil, fmt.Errorf("left feed name is empty")
		}
		if entry.Right == "" {
			return nil, fmt.Errorf("right feed name is empty")
		}
		if len(entry.Left) > comparisonPairLedgerMaxNameBytes {
			return nil, fmt.Errorf("left feed name %q is too long", entry.Left)
		}
		if len(entry.Right) > comparisonPairLedgerMaxNameBytes {
			return nil, fmt.Errorf("right feed name %q is too long", entry.Right)
		}
		if !entry.LeftHashValid && !comparisonPairLedgerHashZero(entry.LeftHash) {
			return nil, fmt.Errorf("left hash for %s/%s is marked invalid but is non-zero", entry.Left, entry.Right)
		}
		if !entry.RightHashValid && !comparisonPairLedgerHashZero(entry.RightHash) {
			return nil, fmt.Errorf("right hash for %s/%s is marked invalid but is non-zero", entry.Left, entry.Right)
		}
		if _, ok := nameSet[entry.Left]; !ok {
			nameSet[entry.Left] = struct{}{}
			names = append(names, entry.Left)
		}
		if _, ok := nameSet[entry.Right]; !ok {
			nameSet[entry.Right] = struct{}{}
			names = append(names, entry.Right)
		}
	}
	sort.Strings(names)
	if uint64(len(names)) > uint64(^uint32(0)) {
		return nil, fmt.Errorf("comparison pair ledger has too many feed names: %d", len(names))
	}
	nameIndex := make(map[string]uint32, len(names))
	size := comparisonPairLedgerHeaderBytes + len(disk.Entries)*comparisonPairLedgerEntryFixedBytes
	for idx, name := range names {
		nameIndex[name] = uint32(idx)
		size += 2 + len(name)
	}
	data := make([]byte, 0, size)
	data = append(data, comparisonPairLedgerMagic[:]...)
	data = binary.LittleEndian.AppendUint16(data, uint16(disk.Version))
	data = binary.LittleEndian.AppendUint16(data, uint16(disk.AlgorithmVersion))
	data = binary.LittleEndian.AppendUint32(data, uint32(len(names)))
	data = binary.LittleEndian.AppendUint64(data, uint64(len(disk.Entries)))
	for _, name := range names {
		data = binary.LittleEndian.AppendUint16(data, uint16(len(name)))
		data = append(data, name...)
	}
	for _, entry := range disk.Entries {
		data = binary.LittleEndian.AppendUint32(data, nameIndex[entry.Left])
		data = binary.LittleEndian.AppendUint32(data, nameIndex[entry.Right])
		var flags byte
		if entry.LeftHashValid {
			flags |= 1
		}
		if entry.RightHashValid {
			flags |= 2
		}
		data = append(data, flags)
		data = binary.LittleEndian.AppendUint64(data, entry.Common)
		data = append(data, entry.LeftHash[:]...)
		data = append(data, entry.RightHash[:]...)
	}
	return data, nil
}

func parseComparisonPairLedgerFile(data []byte) (comparisonPairLedgerFile, error) {
	if len(data) < comparisonPairLedgerHeaderBytes {
		return comparisonPairLedgerFile{}, errComparisonPairLedgerTooShort
	}
	if !bytes.Equal(data[:len(comparisonPairLedgerMagic)], comparisonPairLedgerMagic[:]) {
		return comparisonPairLedgerFile{}, fmt.Errorf("comparison pair ledger magic mismatch")
	}
	version := int(binary.LittleEndian.Uint16(data[8:10]))
	if version != comparisonPairLedgerFormatVersion {
		return comparisonPairLedgerFile{}, fmt.Errorf("comparison pair ledger version %d does not match %d", version, comparisonPairLedgerFormatVersion)
	}
	algorithmVersion := int(binary.LittleEndian.Uint16(data[10:12]))
	if algorithmVersion != comparisonPairLedgerAlgorithmVersion {
		return comparisonPairLedgerFile{}, fmt.Errorf("comparison pair ledger algorithm version %d does not match %d", algorithmVersion, comparisonPairLedgerAlgorithmVersion)
	}
	feedCount := binary.LittleEndian.Uint32(data[12:16])
	entryCount := binary.LittleEndian.Uint64(data[16:24])
	if uint64(feedCount) > uint64(len(data)/3+1) {
		return comparisonPairLedgerFile{}, fmt.Errorf("comparison pair ledger feed count %d exceeds file size %d", feedCount, len(data))
	}
	if entryCount > uint64(len(data)/comparisonPairLedgerEntryFixedBytes+1) {
		return comparisonPairLedgerFile{}, fmt.Errorf("comparison pair ledger entry count %d exceeds file size %d", entryCount, len(data))
	}
	disk := comparisonPairLedgerFile{
		Version:          version,
		AlgorithmVersion: algorithmVersion,
		Entries:          make([]comparisonPairLedgerEntry, 0, int(entryCount)),
	}
	offset := comparisonPairLedgerHeaderBytes
	names := make([]string, int(feedCount))
	for idx := uint32(0); idx < feedCount; idx++ {
		if len(data)-offset < 2 {
			return comparisonPairLedgerFile{}, fmt.Errorf("feed name %d: %w", idx, errComparisonPairLedgerTooShort)
		}
		nameLen := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2
		if nameLen == 0 {
			return comparisonPairLedgerFile{}, fmt.Errorf("feed name %d is empty", idx)
		}
		if len(data)-offset < nameLen {
			return comparisonPairLedgerFile{}, fmt.Errorf("feed name %d: %w", idx, errComparisonPairLedgerTooShort)
		}
		names[idx] = string(data[offset : offset+nameLen])
		offset += nameLen
	}
	for idx := uint64(0); idx < entryCount; idx++ {
		if len(data)-offset < comparisonPairLedgerEntryFixedBytes {
			return comparisonPairLedgerFile{}, fmt.Errorf("entry %d: %w", idx, errComparisonPairLedgerTooShort)
		}
		leftIdx := binary.LittleEndian.Uint32(data[offset : offset+4])
		rightIdx := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		flags := data[offset+8]
		common := binary.LittleEndian.Uint64(data[offset+9 : offset+17])
		offset += 17
		var leftHash [32]byte
		copy(leftHash[:], data[offset:offset+32])
		offset += 32
		var rightHash [32]byte
		copy(rightHash[:], data[offset:offset+32])
		offset += 32
		if int(leftIdx) >= len(names) {
			return comparisonPairLedgerFile{}, fmt.Errorf("entry %d left feed index %d exceeds feed count %d", idx, leftIdx, len(names))
		}
		if int(rightIdx) >= len(names) {
			return comparisonPairLedgerFile{}, fmt.Errorf("entry %d right feed index %d exceeds feed count %d", idx, rightIdx, len(names))
		}
		left := names[leftIdx]
		right := names[rightIdx]
		leftHashValid := flags&1 != 0
		rightHashValid := flags&2 != 0
		if flags&^byte(3) != 0 {
			return comparisonPairLedgerFile{}, fmt.Errorf("entry %d has unsupported flags %d", idx, flags)
		}
		if !leftHashValid && !comparisonPairLedgerHashZero(leftHash) {
			return comparisonPairLedgerFile{}, fmt.Errorf("entry %d left hash is invalid but non-zero", idx)
		}
		if !rightHashValid && !comparisonPairLedgerHashZero(rightHash) {
			return comparisonPairLedgerFile{}, fmt.Errorf("entry %d right hash is invalid but non-zero", idx)
		}
		disk.Entries = append(disk.Entries, comparisonPairLedgerEntry{
			Left:           left,
			Right:          right,
			LeftHash:       leftHash,
			LeftHashValid:  leftHashValid,
			RightHash:      rightHash,
			RightHashValid: rightHashValid,
			Common:         common,
		})
	}
	if offset != len(data) {
		return comparisonPairLedgerFile{}, fmt.Errorf("comparison pair ledger has %d trailing bytes", len(data)-offset)
	}
	return disk, nil
}

func comparisonPairLedgerKeyForInfos(left, right comparisonSetInfo) comparisonPairLedgerKey {
	leftHash := comparisonPairLedgerHash{
		sum:   left.contentHash.Sum,
		valid: left.contentHash.Valid,
	}
	rightHash := comparisonPairLedgerHash{
		sum:   right.contentHash.Sum,
		valid: right.contentHash.Valid,
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
		LeftHash:       key.leftHash.sum,
		LeftHashValid:  key.leftHash.valid,
		RightHash:      key.rightHash.sum,
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
	if !e.LeftHashValid && !comparisonPairLedgerHashZero(e.LeftHash) {
		return comparisonPairLedgerKey{}, fmt.Errorf("left hash is invalid but non-zero")
	}
	if !e.RightHashValid && !comparisonPairLedgerHashZero(e.RightHash) {
		return comparisonPairLedgerKey{}, fmt.Errorf("right hash is invalid but non-zero")
	}
	key := comparisonPairLedgerKey{
		left:             e.Left,
		right:            e.Right,
		leftHash:         comparisonPairLedgerHash{sum: e.LeftHash, valid: e.LeftHashValid},
		rightHash:        comparisonPairLedgerHash{sum: e.RightHash, valid: e.RightHashValid},
		algorithmVersion: comparisonPairLedgerAlgorithmVersion,
	}
	if key.right < key.left {
		key.left, key.right = key.right, key.left
		key.leftHash, key.rightHash = key.rightHash, key.leftHash
	}
	return key, nil
}

func (e comparisonPairLegacyLedgerEntry) key() (comparisonPairLedgerKey, error) {
	if e.Left == "" {
		return comparisonPairLedgerKey{}, fmt.Errorf("left feed name is empty")
	}
	if e.Right == "" {
		return comparisonPairLedgerKey{}, fmt.Errorf("right feed name is empty")
	}
	leftHash, err := decodeComparisonPairLegacyLedgerHash(e.LeftHash, e.LeftHashValid)
	if err != nil {
		return comparisonPairLedgerKey{}, fmt.Errorf("left hash: %w", err)
	}
	rightHash, err := decodeComparisonPairLegacyLedgerHash(e.RightHash, e.RightHashValid)
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

func comparisonPairLedgerHashZero(hash [32]byte) bool {
	return hash == [32]byte{}
}

func decodeComparisonPairLegacyLedgerHash(value string, valid bool) (comparisonPairLedgerHash, error) {
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
