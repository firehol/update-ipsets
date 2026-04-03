package processor

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

func normalizeCommented(input []byte, comment string) []byte {
	lines := splitLines(bytes.ReplaceAll(input, []byte("\r"), []byte("\n")))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if comment != "" {
			if idx := strings.Index(line, comment); idx >= 0 {
				line = line[:idx]
			}
		}
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	return joinLines(out)
}

func splitLines(input []byte) []string {
	input = stripBOM(input)
	text := strings.ReplaceAll(string(input), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.Split(text, "\n")
}

// stripBOM removes a UTF-8 BOM (byte order mark) from the start of input.
// Many Windows-origin feeds include a BOM prefix that would otherwise
// corrupt the first line during parsing.
func stripBOM(data []byte) []byte {
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return data[3:]
	}
	return data
}

func joinLines(lines []string) []byte {
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			filtered = append(filtered, line)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return []byte(strings.Join(filtered, "\n") + "\n")
}

// xmlTagCache caches compiled regexes for extractXMLTagContent to avoid
// recompilation on each call for the same tag.
var xmlTagCache sync.Map

func extractXMLTagContent(input, tag string) []string {
	pattern := fmt.Sprintf(`(?is)<%s[^>]*>(.*?)</%s>`, regexp.QuoteMeta(tag), regexp.QuoteMeta(tag))
	var re *regexp.Regexp
	if cached, ok := xmlTagCache.Load(pattern); ok {
		re = cached.(*regexp.Regexp)
	} else {
		re = regexp.MustCompile(pattern)
		xmlTagCache.Store(pattern, re)
	}
	matches := re.FindAllStringSubmatch(input, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		value := strings.TrimSpace(htmlUnescape(match[1]))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func htmlUnescape(s string) string {
	replacer := strings.NewReplacer(
		"&lt;", "<",
		"&gt;", ">",
		"&amp;", "&",
		"&quot;", `"`,
		"&#39;", "'",
	)
	return replacer.Replace(s)
}

func findIPv4(input string) []string {
	matches := ipv4Pattern.FindAllStringSubmatch(input, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return out
}

// findIPv4CIDR is findIPv4's CIDR-preserving sibling. It uses
// ipv4CIDRPattern so each extracted token keeps its /prefix if
// one was present in the input. Used by the extract_ipv4_cidr
// processor.
func findIPv4CIDR(input string) []string {
	matches := ipv4CIDRPattern.FindAllStringSubmatch(input, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return out
}

func unzipFileByName(input []byte, want string) ([]byte, error) {
	readerAt := bytes.NewReader(input)
	archive, err := zip.NewReader(readerAt, int64(len(input)))
	if err != nil {
		return nil, err
	}
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if want != "" && file.Name != want {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer func() { _ = rc.Close() }()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("zip file entry %q not found", want)
}

func extractP2PRanges(input []byte, onlyProxy bool) []byte {
	lines := splitLines(input)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if onlyProxy && !strings.HasPrefix(line, "Proxy") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		if rangePattern.MatchString(value) {
			out = append(out, value)
		}
	}
	return joinLines(out)
}

func numericIPv4(s string) (string, bool) {
	s = strings.Trim(s, `"`)
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return "", false
	}
	ip := net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	return ip.String(), true
}
