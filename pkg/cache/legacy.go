package cache

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/firehol/update-ipsets/internal/fileutil"
)

func LoadWithMigration(jsonPath, legacyPath string) (*State, error) {
	st, err := Load(jsonPath)
	if err != nil {
		return nil, err
	}
	if len(st.Entries) > 0 || !fileExists(legacyPath) {
		return st, nil
	}
	migrated, err := LoadLegacy(legacyPath)
	if err != nil {
		return nil, err
	}
	if err := Save(jsonPath, migrated); err != nil {
		return nil, err
	}
	return migrated, nil
}

func LoadLegacy(path string) (*State, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	st := New()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "declare -A ") {
			continue
		}
		name, values, ok, err := parseLegacyDeclare(line)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		for key, value := range values {
			applyLegacyValue(st.Entry(key), name, value)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return st, nil
}

func parseLegacyDeclare(line string) (string, map[string]string, bool, error) {
	const prefix = "declare -A "
	if !strings.HasPrefix(line, prefix) {
		return "", nil, false, nil
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	name, raw, ok := strings.Cut(rest, "=")
	if !ok {
		return "", nil, false, fmt.Errorf("invalid legacy cache line %q", line)
	}
	name = strings.TrimSpace(name)
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "(") || !strings.HasSuffix(raw, ")") {
		return "", nil, false, fmt.Errorf("invalid associative array payload for %s", name)
	}
	values, err := parseLegacyAssocArray(strings.TrimSuffix(strings.TrimPrefix(raw, "("), ")"))
	if err != nil {
		return "", nil, false, err
	}
	return name, values, true, nil
}

func parseLegacyAssocArray(raw string) (map[string]string, error) {
	out := map[string]string{}
	for i := 0; i < len(raw); {
		for i < len(raw) && isShellSpace(raw[i]) {
			i++
		}
		if i >= len(raw) {
			break
		}
		if raw[i] != '[' {
			return nil, fmt.Errorf("expected '[' in associative array near %q", raw[i:])
		}
		i++
		keyStart := i
		for i < len(raw) && raw[i] != ']' {
			i++
		}
		if i >= len(raw) {
			return nil, fmt.Errorf("unterminated associative array key")
		}
		key := raw[keyStart:i]
		i++
		if i >= len(raw) || raw[i] != '=' {
			return nil, fmt.Errorf("missing '=' for associative array key %q", key)
		}
		i++
		value, next, err := parseLegacyShellValue(raw, i)
		if err != nil {
			return nil, err
		}
		out[key] = value
		i = next
	}
	return out, nil
}

func parseLegacyShellValue(raw string, start int) (string, int, error) {
	i := start
	for i < len(raw) && isShellSpace(raw[i]) {
		i++
	}
	if i >= len(raw) {
		return "", i, nil
	}
	switch raw[i] {
	case '"':
		// Manual unquote: bash double-quoted strings can contain literal
		// newlines and other characters that strconv.Unquote rejects.
		j := i + 1
		var b strings.Builder
		for j < len(raw) {
			if raw[j] == '"' {
				return b.String(), j + 1, nil
			}
			if raw[j] == '\\' && j+1 < len(raw) {
				j++
				switch raw[j] {
				case '"', '\\', '$', '`':
					b.WriteByte(raw[j])
				case 'n':
					b.WriteByte('\n')
				case 'r':
					b.WriteByte('\r')
				case 't':
					b.WriteByte('\t')
				default:
					// Bash only escapes specific chars in double quotes;
					// for others, the backslash is kept literally.
					b.WriteByte('\\')
					b.WriteByte(raw[j])
				}
				j++
				continue
			}
			b.WriteByte(raw[j])
			j++
		}
		return "", 0, fmt.Errorf("unterminated double-quoted value")
	case '\'':
		j := i + 1
		for j < len(raw) && raw[j] != '\'' {
			j++
		}
		if j >= len(raw) {
			return "", 0, fmt.Errorf("unterminated single-quoted value")
		}
		return raw[i+1 : j], j + 1, nil
	case '$':
		if i+1 < len(raw) && raw[i+1] == '\'' {
			j := i + 2
			var b strings.Builder
			for j < len(raw) {
				if raw[j] == '\'' {
					return b.String(), j + 1, nil
				}
				if raw[j] == '\\' && j+1 < len(raw) {
					j++
					switch raw[j] {
					case 'n':
						b.WriteByte('\n')
						j++
					case 'r':
						b.WriteByte('\r')
						j++
					case 't':
						b.WriteByte('\t')
						j++
					case '\\', '\'', '"':
						b.WriteByte(raw[j])
						j++
					case '0', '1', '2', '3', '4', '5', '6', '7':
						// Octal escape: \NNN (1-3 digits)
						val := int(raw[j] - '0')
						j++
						for k := 0; k < 2 && j < len(raw) && raw[j] >= '0' && raw[j] <= '7'; k++ {
							val = val*8 + int(raw[j]-'0')
							j++
						}
						b.WriteByte(byte(val))
					case 'x':
						// Hex escape: \xNN (1-2 digits)
						j++
						val := 0
						for k := 0; k < 2 && j < len(raw) && isHexDigit(raw[j]); k++ {
							val = val*16 + hexVal(raw[j])
							j++
						}
						b.WriteByte(byte(val))
					default:
						b.WriteByte(raw[j])
						j++
					}
					continue
				}
				b.WriteByte(raw[j])
				j++
			}
			return "", 0, fmt.Errorf("unterminated ANSI-C quoted value")
		}
	}
	j := i
	for j < len(raw) && !isShellSpace(raw[j]) {
		j++
	}
	return raw[i:j], j, nil
}

func applyLegacyValue(entry *Entry, name, value string) {
	switch name {
	case "IPSET_INFO":
		entry.Info = value
	case "IPSET_SOURCE":
		entry.Source = value
	case "IPSET_URL":
		entry.URL = value
	case "IPSET_FILE":
		entry.File = value
	case "IPSET_IPV":
		entry.IPV = value
	case "IPSET_HASH":
		entry.Hash = value
	case "IPSET_MINS":
		entry.FrequencyMinutes = parseLegacyInt(value)
	case "IPSET_HISTORY_MINS":
		entry.HistoryMinutes = parseLegacyIntList(value)
	case "IPSET_ENTRIES":
		entry.Entries = parseLegacyInt(value)
	case "IPSET_IPS":
		entry.UniqueIPs = parseLegacyUint(value)
	case "IPSET_SOURCE_DATE":
		entry.SourceDate = parseLegacyInt64(value)
	case "IPSET_CHECKED_DATE":
		entry.CheckedDate = parseLegacyInt64(value)
	case "IPSET_PROCESSED_DATE":
		entry.ProcessedDate = parseLegacyInt64(value)
	case "IPSET_STARTED_DATE":
		entry.StartedDate = parseLegacyInt64(value)
	case "IPSET_CATEGORY":
		entry.Category = value
	case "IPSET_MAINTAINER":
		entry.Maintainer = value
	case "IPSET_MAINTAINER_URL":
		entry.MaintainerURL = value
	case "IPSET_ENTRIES_MIN":
		entry.EntriesMin = parseLegacyInt(value)
	case "IPSET_ENTRIES_MAX":
		entry.EntriesMax = parseLegacyInt(value)
	case "IPSET_IPS_MIN":
		entry.IPsMin = parseLegacyUint(value)
	case "IPSET_IPS_MAX":
		entry.IPsMax = parseLegacyUint(value)
	case "IPSET_CLOCK_SKEW":
		entry.ClockSkewSeconds = parseLegacyInt64(value)
	case "IPSET_DOWNLOAD_FAILURES":
		entry.DownloadFailures = parseLegacyInt(value)
	case "IPSET_VERSION":
		entry.Version = parseLegacyInt(value)
	case "IPSET_AVERAGE_UPDATE_TIME":
		entry.AverageUpdateMins = parseLegacyInt(value)
	case "IPSET_MIN_UPDATE_TIME":
		entry.MinUpdateMins = parseLegacyInt(value)
	case "IPSET_MAX_UPDATE_TIME":
		entry.MaxUpdateMins = parseLegacyInt(value)
	case "IPSET_DOWNLOADER":
		entry.Downloader = value
	case "IPSET_DOWNLOADER_OPTIONS":
		entry.DownloaderOptions = value
	}
}

func parseLegacyInt(raw string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(raw))
	return n
}

func parseLegacyInt64(raw string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return n
}

func parseLegacyUint(raw string) uint64 {
	n, _ := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
	return n
}

func parseLegacyIntList(raw string) []int {
	fields := strings.Fields(raw)
	out := make([]int, 0, len(fields))
	for _, field := range fields {
		n, err := strconv.Atoi(field)
		if err == nil {
			out = append(out, n)
		}
	}
	return out
}

func isShellSpace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func hexVal(ch byte) int {
	switch {
	case ch >= '0' && ch <= '9':
		return int(ch - '0')
	case ch >= 'a' && ch <= 'f':
		return int(ch-'a') + 10
	case ch >= 'A' && ch <= 'F':
		return int(ch-'A') + 10
	}
	return 0
}

func fileExists(path string) bool {
	return fileutil.Exists(path)
}
