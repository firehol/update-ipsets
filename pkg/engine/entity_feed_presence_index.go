package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	entityFeedPresenceIndexFileName     = "feed-presence-v1.bin"
	entityFeedPresenceIndexFormat       = uint32(1)
	entityFeedPresenceIndexHeaderBytes  = 16
	entityFeedPresenceIndexMaxBytes     = 8 << 20
	entityFeedPresenceIndexMaxNameBytes = 1<<16 - 1
)

var (
	entityFeedPresenceIndexMagic   = [8]byte{'U', 'I', 'P', 'E', 'F', 'P', 'I', 'X'}
	errEntityFeedPresenceIndexMiss = errors.New("entity feed presence index missing")
)

func (e *Engine) entityFeedPresenceIndexPath() string {
	return filepath.Join(e.entitiesDir(), entityFeedPresenceIndexFileName)
}

func (e *Engine) entityFeedPresenceIndexRelPath() string {
	return entityFeedPresenceIndexFileName
}

func (e *Engine) loadEntityFeedPresenceIndex() (map[string]struct{}, int64, error) {
	path := e.entityFeedPresenceIndexPath()
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, errEntityFeedPresenceIndexMiss
		}
		return nil, 0, fmt.Errorf("stat entity feed presence index: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("entity feed presence index is not a regular file")
	}
	if info.Size() > entityFeedPresenceIndexMaxBytes {
		return nil, 0, fmt.Errorf("entity feed presence index is too large: %d bytes", info.Size())
	}
	data, err := readFileInRoot(e.entitiesDir(), entityFeedPresenceIndexFileName)
	if err != nil {
		return nil, 0, fmt.Errorf("read entity feed presence index: %w", err)
	}
	names, err := parseEntityFeedPresenceIndex(data)
	if err != nil {
		return nil, int64(len(data)), fmt.Errorf("decode entity feed presence index: %w", err)
	}
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		out[name] = struct{}{}
	}
	return out, int64(len(data)), nil
}

func stageEntityFeedPresenceIndex(entityBatch *stagedPublishBatch, names []string) error {
	if entityBatch == nil {
		return nil
	}
	data, err := marshalEntityFeedPresenceIndex(names)
	if err != nil {
		return err
	}
	return writeFileAtomicNoSync(filepath.Join(entityBatch.stageDir, entityFeedPresenceIndexFileName), data, generatedFileMode)
}

func marshalEntityFeedPresenceIndex(names []string) ([]byte, error) {
	names = uniqueNonEmptyStrings(names)
	size := entityFeedPresenceIndexHeaderBytes
	for _, name := range names {
		if len(name) > entityFeedPresenceIndexMaxNameBytes {
			return nil, fmt.Errorf("entity feed presence index name %q is too long", name)
		}
		size += 2 + len(name)
	}
	data := make([]byte, 0, size)
	data = append(data, entityFeedPresenceIndexMagic[:]...)
	data = binary.LittleEndian.AppendUint32(data, entityFeedPresenceIndexFormat)
	data = binary.LittleEndian.AppendUint32(data, uint32(len(names)))
	for _, name := range names {
		data = binary.LittleEndian.AppendUint16(data, uint16(len(name)))
		data = append(data, name...)
	}
	return data, nil
}

func parseEntityFeedPresenceIndex(data []byte) ([]string, error) {
	if len(data) < entityFeedPresenceIndexHeaderBytes {
		return nil, fmt.Errorf("entity feed presence index is truncated")
	}
	if !bytes.Equal(data[:len(entityFeedPresenceIndexMagic)], entityFeedPresenceIndexMagic[:]) {
		return nil, fmt.Errorf("entity feed presence index magic mismatch")
	}
	version := binary.LittleEndian.Uint32(data[8:12])
	if version != entityFeedPresenceIndexFormat {
		return nil, fmt.Errorf("entity feed presence index version %d does not match %d", version, entityFeedPresenceIndexFormat)
	}
	count := binary.LittleEndian.Uint32(data[12:16])
	if count > uint32(len(data)/2) {
		return nil, fmt.Errorf("entity feed presence index count %d exceeds file size %d", count, len(data))
	}
	names := make([]string, 0, count)
	seen := make(map[string]struct{}, count)
	offset := entityFeedPresenceIndexHeaderBytes
	for idx := uint32(0); idx < count; idx++ {
		if len(data)-offset < 2 {
			return nil, fmt.Errorf("entity feed presence index name %d is truncated", idx)
		}
		nameLen := int(binary.LittleEndian.Uint16(data[offset : offset+2]))
		offset += 2
		if nameLen == 0 {
			return nil, fmt.Errorf("entity feed presence index name %d is empty", idx)
		}
		if len(data)-offset < nameLen {
			return nil, fmt.Errorf("entity feed presence index name %d is truncated", idx)
		}
		name := string(data[offset : offset+nameLen])
		offset += nameLen
		if strings.TrimSpace(name) != name {
			return nil, fmt.Errorf("entity feed presence index name %d is not normalized", idx)
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("entity feed presence index duplicate name %q", name)
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	if len(data) != offset {
		return nil, fmt.Errorf("entity feed presence index has %d trailing bytes", len(data)-offset)
	}
	if !slices.IsSorted(names) {
		return nil, fmt.Errorf("entity feed presence index names are not sorted")
	}
	return names, nil
}

func entityFeedPresenceNamesFromSidecars(sidecars map[string]*feedEntitySidecar) []string {
	names := make([]string, 0, len(sidecars))
	for name, sidecar := range sidecars {
		if !feedEntitySidecarHasEntityPresence(sidecar) {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = strings.TrimSpace(sidecar.Feed)
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return uniqueNonEmptyStrings(names)
}

func feedEntitySidecarHasEntityPresence(sidecar *feedEntitySidecar) bool {
	if sidecar == nil {
		return false
	}
	return len(sidecar.countryCodes()) > 0 || len(sidecar.asnNumbers()) > 0
}
