package processor

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// StreamFunc transforms an input reader into an output reader.
// The caller is responsible for closing the returned reader (if it implements io.Closer).
type StreamFunc func(r io.Reader, args map[string]string) (io.Reader, error)

// streamRegistry maps processor names to their streaming implementations.
// Only processors that can work line-by-line (or as streaming decompressors)
// are registered here. Processors that need full materialization (hostname_resolve,
// json_path, xml-based, regex on whole input) are NOT registered.
var streamRegistry = map[string]StreamFunc{}

func init() {
	for name, fn := range map[string]StreamFunc{
		// passthrough
		"passthrough": streamPassthrough,
		"cat":         streamPassthrough,
		"$CAT_CMD":    streamPassthrough,

		// comment removal
		"remove_comments":            streamRemoveHashComments,
		"remove_comments_semi":       streamRemoveSemiComments,
		"remove_comments_semi_colon": streamRemoveSemiComments,

		// whitespace
		"trim": streamTrim,

		// line filters
		"grep":     streamGrep,
		"grep_not": streamGrepNot,

		// field extraction
		"cut_delimiter":          streamCutDelimiter,
		"csv_column":             streamCSVColumn,
		"csv_comma_first_column": streamCSVColumn,

		// IP extraction/transform
		"extract_ipv4":                    streamExtractIPv4,
		"extract_ipv4_from_any_file":      streamExtractIPv4,
		"extract_ipv4_cidr":               streamExtractIPv4CIDR,
		"extract_cidr":                    streamExtractIPv4CIDR,
		"extract_ipv4_cidr_from_any_file": streamExtractIPv4CIDR,
		"subnet_to_cidr":                  streamSubnetToCIDR,
		"subnet_to_bitmask":               streamSubnetToCIDR,

		// torproject
		"torproject_exits": streamTorprojectExits,

		// suffix operations
		"remove_slash32":  streamRemoveSlash32,
		"append_slash32":  streamAppendSlash32,
		"remove_slash128": streamRemoveSlash128,
		"append_slash128": streamAppendSlash128,

		// IP filters
		"filter_ip4":      streamFilterIP4,
		"filter_net4":     streamFilterNet4,
		"filter_all4":     streamFilterAll4,
		"filter_invalid4": streamFilterInvalid4,
		"filter_ip6":      streamFilterIP6,
		"filter_net6":     streamFilterNet6,
		"filter_all6":     streamFilterAll6,

		// snort/pix
		"snort_rules":               streamSnortRules,
		"snort_alert_rules_to_ipv4": streamSnortRules,
		"pix_deny_rules":            streamPixDenyRules,
		"pix_deny_rules_to_ipv4":    streamPixDenyRules,

		// dshield line format
		"dshield_format": streamDshieldFormat,
		"dshield_parser": streamDshieldFormat,

		// dataplane
		"dataplane_column3": streamDataplaneColumn3,

		// p2p gzip blocklists
		"p2p_blocklist":       streamP2PBlocklist,
		"p2p_gz":              streamP2PBlocklist,
		"p2p_blocklist_ips":   streamP2PBlocklist,
		"p2p_gz_ips":          streamP2PBlocklist,
		"p2p_blocklist_proxy": streamP2PBlocklistProxy,
		"p2p_gz_proxy":        streamP2PBlocklistProxy,

		// decompression (streamable via gzip.NewReader)
		"gunzip": streamGunzip,
	} {
		streamRegistry[name] = fn
	}
}

type closerReader struct {
	io.Reader
	closer io.Closer
}

func (r closerReader) Close() error {
	return r.closer.Close()
}

// IsStreamable returns true if every step in the pipeline has a streaming implementation.
func IsStreamable(steps []string) bool {
	for _, name := range steps {
		if name == "" {
			name = "passthrough"
		}
		if _, ok := streamRegistry[name]; !ok {
			return false
		}
	}
	return true
}

// lineFilterReader wraps a bufio.Reader, applying a per-line filter function.
// It implements io.Reader by lazily reading and filtering lines on demand
// without Scanner token-size limits.
type lineFilterReader struct {
	reader    *bufio.Reader
	filter    func(string) []string // returns zero or more output lines per input line
	buf       []byte                // buffered output not yet returned
	done      bool
	firstLine bool // true until the first line has been read
}

func newLineFilterReader(r io.Reader, filter func(string) []string) *lineFilterReader {
	return &lineFilterReader{
		reader:    bufio.NewReaderSize(r, 64*1024),
		filter:    filter,
		firstLine: true,
	}
}

func (lfr *lineFilterReader) Read(p []byte) (int, error) {
	for len(lfr.buf) == 0 && !lfr.done {
		line, err := lfr.reader.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			if len(lfr.buf) > 0 {
				break
			}
			return 0, err
		}
		if errors.Is(err, io.EOF) {
			lfr.done = true
		}
		if len(line) == 0 {
			if lfr.done {
				return 0, io.EOF
			}
			continue
		}
		// Strip UTF-8 BOM from the first line of the stream.
		if lfr.firstLine {
			lfr.firstLine = false
			line = []byte(strings.TrimPrefix(string(line), "\xEF\xBB\xBF"))
		}
		// normalize \r
		text := strings.TrimRight(string(line), "\r\n")
		results := lfr.filter(text)
		for _, result := range results {
			lfr.buf = append(lfr.buf, result...)
			lfr.buf = append(lfr.buf, '\n')
		}
	}
	if len(lfr.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, lfr.buf)
	lfr.buf = lfr.buf[n:]
	return n, nil
}

// --- Streaming processor implementations ---

func streamPassthrough(r io.Reader, _ map[string]string) (io.Reader, error) {
	return r, nil
}

func streamRemoveHashComments(r io.Reader, _ map[string]string) (io.Reader, error) {
	return streamRemoveComments(r, "#"), nil
}

func streamRemoveSemiComments(r io.Reader, _ map[string]string) (io.Reader, error) {
	return streamRemoveComments(r, ";"), nil
}

func streamRemoveComments(r io.Reader, comment string) io.Reader {
	return newLineFilterReader(r, func(line string) []string {
		if comment != "" {
			if idx := strings.Index(line, comment); idx >= 0 {
				line = line[:idx]
			}
		}
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			return nil
		}
		return []string{line}
	})
}

func streamP2PBlocklist(r io.Reader, _ map[string]string) (io.Reader, error) {
	return streamP2PBlocklistCommon(r, false)
}

func streamP2PBlocklistProxy(r io.Reader, _ map[string]string) (io.Reader, error) {
	return streamP2PBlocklistCommon(r, true)
}

func streamP2PBlocklistCommon(r io.Reader, onlyProxy bool) (io.Reader, error) {
	decompressed, err := streamGunzip(r, nil)
	if err != nil {
		return nil, err
	}
	filtered := newLineFilterReader(decompressed, func(line string) []string {
		line = strings.TrimSpace(line)
		if line == "" {
			return nil
		}
		if onlyProxy && !strings.HasPrefix(line, "Proxy") {
			return nil
		}
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			return nil
		}
		value := strings.TrimSpace(parts[1])
		if !rangePattern.MatchString(value) {
			return nil
		}
		return []string{value}
	})
	if closer, ok := decompressed.(io.Closer); ok {
		return closerReader{Reader: filtered, closer: closer}, nil
	}
	return filtered, nil
}

func streamTrim(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			return nil
		}
		return []string{line}
	}), nil
}

func streamGrep(r io.Reader, args map[string]string) (io.Reader, error) {
	return streamGrepCommon(r, args, false)
}

func streamGrepNot(r io.Reader, args map[string]string) (io.Reader, error) {
	return streamGrepCommon(r, args, true)
}

func streamGrepCommon(r io.Reader, args map[string]string, invert bool) (io.Reader, error) {
	pattern := strings.TrimSpace(args["pattern"])
	if pattern == "" {
		pattern = strings.TrimSpace(args["value"])
	}
	if pattern == "" {
		return nil, fmt.Errorf("missing grep pattern")
	}
	if args["literal"] != "" {
		pattern = regexp.QuoteMeta(pattern)
	}
	if args["case_insensitive"] != "" {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return newLineFilterReader(r, func(line string) []string {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			return nil
		}
		matched := re.MatchString(line)
		if invert {
			matched = !matched
		}
		if matched {
			return []string{line}
		}
		return nil
	}), nil
}

func streamCutDelimiter(r io.Reader, args map[string]string) (io.Reader, error) {
	delimiter := args["delimiter"]
	if delimiter == "" {
		delimiter = args["value"]
	}
	if delimiter == "" {
		delimiter = " "
	}
	field := 1
	if raw := strings.TrimSpace(args["field"]); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid cut field %q", raw)
		}
		field = n
	}
	return newLineFilterReader(r, func(line string) []string {
		if line == "" {
			return nil
		}
		parts := strings.Split(line, delimiter)
		if field > len(parts) {
			return nil
		}
		value := strings.TrimSpace(parts[field-1])
		if value == "" {
			return nil
		}
		return []string{value}
	}), nil
}

func streamCSVColumn(r io.Reader, args map[string]string) (io.Reader, error) {
	index := 1
	if raw := strings.TrimSpace(args["index"]); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid csv column index %q", raw)
		}
		index = n
	}
	// CSV reader needs the entire reader but we can stream record-by-record
	pr, pw := io.Pipe()
	go func() {
		defer func() { _ = pw.Close() }()
		reader := csv.NewReader(r)
		reader.FieldsPerRecord = -1
		// Skip lines starting with `#` so commented header / banner
		// lines never reach the CSV state machine. The griffinguard
		// feed prefixes its 18-line preamble with `# `, and one of
		// those preamble lines contains bare quotes that the strict
		// CSV reader rejects. Treating `#` as a comment character
		// is the same convention `remove_comments` uses elsewhere
		// in the pipeline.
		reader.Comment = '#'
		// LazyQuotes lets fields contain bare `"` characters without
		// being interpreted as field-quoting. Real-world CSV feeds
		// regularly emit unescaped quotes inside human-readable
		// values (descriptions, classifications, free text), and
		// rejecting the entire feed for one stray quote in row N is
		// the same kind of brittleness the lenient iprange parser
		// already removed.
		reader.LazyQuotes = true
		for {
			row, err := reader.Read()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					pw.CloseWithError(err)
				}
				return
			}
			if len(row) < index {
				continue
			}
			value := strings.TrimSpace(row[index-1])
			if value != "" {
				if _, werr := pw.Write([]byte(value + "\n")); werr != nil {
					return
				}
			}
		}
	}()
	return pr, nil
}

func streamExtractIPv4(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		matches := ipv4Pattern.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			return nil
		}
		out := make([]string, 0, len(matches))
		for _, match := range matches {
			out = append(out, match[1])
		}
		return out
	}), nil
}

// streamExtractIPv4CIDR is streamExtractIPv4's CIDR-preserving
// sibling. See extractIPv4CIDR in processor.go for the rationale —
// used by MISP warninglists and other JSON / text feeds whose
// published ranges are CIDRs rather than bare IPs.
func streamExtractIPv4CIDR(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		matches := ipv4CIDRPattern.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			return nil
		}
		out := make([]string, 0, len(matches))
		for _, match := range matches {
			out = append(out, match[1])
		}
		return out
	}), nil
}

func streamSubnetToCIDR(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		line = strings.TrimSpace(line)
		if line == "" {
			return nil
		}
		parts := strings.Split(line, "/")
		if len(parts) != 2 {
			return []string{line}
		}
		mask := net.ParseIP(strings.TrimSpace(parts[1])).To4()
		if mask == nil {
			return []string{line}
		}
		ones, bits := net.IPMask(mask).Size()
		if bits != 32 {
			return []string{line}
		}
		return []string{strings.TrimSpace(parts[0]) + "/" + strconv.Itoa(ones)}
	}), nil
}

func streamTorprojectExits(r io.Reader, _ map[string]string) (io.Reader, error) {
	return newLineFilterReader(r, func(line string) []string {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "ExitAddress" {
			return []string{fields[1]}
		}
		return nil
	}), nil
}

func streamRemoveSlash32(r io.Reader, _ map[string]string) (io.Reader, error) {
	return streamRewriteSuffix(r, "/32", ""), nil
}

func streamAppendSlash32(r io.Reader, _ map[string]string) (io.Reader, error) {
	return streamAppendSlash(r, "/32"), nil
}

func streamRemoveSlash128(r io.Reader, _ map[string]string) (io.Reader, error) {
	return streamRewriteSuffix(r, "/128", ""), nil
}

func streamAppendSlash128(r io.Reader, _ map[string]string) (io.Reader, error) {
	return streamAppendSlash(r, "/128"), nil
}

func streamRewriteSuffix(r io.Reader, from, to string) io.Reader {
	return newLineFilterReader(r, func(line string) []string {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, from) {
			line = strings.TrimSuffix(line, from) + to
		}
		if line == "" {
			return nil
		}
		return []string{line}
	})
}

func streamAppendSlash(r io.Reader, suffix string) io.Reader {
	return newLineFilterReader(r, func(line string) []string {
		parts := strings.Fields(line)
		if len(parts) == 0 {
			return nil
		}
		value := parts[0]
		if !strings.Contains(value, "/") {
			value += suffix
		}
		return []string{value}
	})
}

// IP filter, format-specific, and decompression streaming processors
// are in stream_filters.go.
