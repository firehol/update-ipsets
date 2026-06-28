package processor

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/firehol/update-ipsets/internal/observability"
	"github.com/firehol/update-ipsets/pkg/config"

	"go.opentelemetry.io/otel/attribute"
)

type stepFunc func(context.Context, []byte, map[string]string) ([]byte, error)

var (
	ipv4Pattern = regexp.MustCompile(`(?:^|[^0-9.])((?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})(?:$|[^0-9.])`)
	// ipv4CIDRPattern is ipv4Pattern with an optional trailing
	// /prefix captured as part of the extracted token. Used by
	// the extract_ipv4_cidr processor to preserve CIDR masks
	// that would otherwise be dropped by ipv4Pattern. The prefix
	// is 0-32 (any valid IPv4 prefix length); entries without a
	// prefix are extracted unchanged as bare IPs.
	//
	// Matching the trailing boundary is trickier: the original
	// pattern uses (?:$|[^0-9.]) which would refuse to extend
	// into the `/` character. We accept `/` into the inner match
	// and then rely on the post-match boundary being "anything
	// that is not `/` or a digit".
	ipv4CIDRPattern = regexp.MustCompile(
		`(?:^|[^0-9./])` +
			`((?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}` +
			`(?:/(?:3[0-2]|[12]?\d))?)` +
			`(?:$|[^0-9./])`,
	)
	rangePattern = regexp.MustCompile(`^((?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}-` +
		`(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})$`)

	// Pre-compiled regexes for processor functions (avoid recompilation per call).
	cleanTalkPattern   = regexp.MustCompile(`>\s*((?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})\s*<`)
	graphicLinePattern = regexp.MustCompile(`>((?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}(?:/(?:3[0-2]|[12]?\d))?)<`)
	botScoutPattern    = regexp.MustCompile(`<a[^>]*?/ipcheck\.htm\?ip=[^>]*>([^<]+)</a>`)
)

var registry = map[string]stepFunc{
	"passthrough":                     passthrough,
	"cat":                             passthrough,
	"$CAT_CMD":                        passthrough,
	"remove_comments":                 removeHashComments,
	"remove_comments_semi":            removeSemiComments,
	"remove_comments_semi_colon":      removeSemiComments,
	"snort_rules":                     snortRules,
	"snort_alert_rules_to_ipv4":       snortRules,
	"pix_deny_rules":                  pixDenyRules,
	"pix_deny_rules_to_ipv4":          pixDenyRules,
	"dshield_format":                  dshieldFormat,
	"dshield_parser":                  dshieldFormat,
	"xml_rss_title_resolve":           xmlRSSTitleResolve,
	"parse_rss_rosinstrument":         xmlRSSTitleResolve,
	"xml_rss_proxy":                   xmlRSSProxy,
	"parse_rss_proxy":                 xmlRSSProxy,
	"xml_rss_title":                   xmlRSSTitle,
	"parse_php_rss":                   xmlRSSTitle,
	"xml_tag":                         xmlTag,
	"parse_xml_clean_mx":              xmlTag,
	"dshield_api_xml":                 dshieldAPIXML,
	"parse_dshield_api":               dshieldAPIXML,
	"subnet_to_cidr":                  subnetToCIDR,
	"subnet_to_bitmask":               subnetToCIDR,
	"extract_ipv4":                    extractIPv4,
	"extract_ipv4_from_any_file":      extractIPv4,
	"extract_ipv4_cidr":               extractIPv4CIDR,
	"extract_cidr":                    extractIPv4CIDR,
	"extract_ipv4_cidr_from_any_file": extractIPv4CIDR,
	"csv_column":                      csvColumn,
	"csv_comma_first_column":          csvColumn,
	"gunzip":                          gunzipFile,
	"unzip_csv":                       unzipCSV,
	"unzip_and_split_csv":             unzipCSV,
	"unzip":                           unzipFirst,
	"unzip_and_extract":               unzipFirst,
	"p2p_blocklist":                   p2pBlocklist,
	"p2p_gz":                          p2pBlocklist,
	"p2p_blocklist_ips":               p2pBlocklist,
	"p2p_gz_ips":                      p2pBlocklist,
	"p2p_blocklist_proxy":             p2pBlocklistProxy,
	"p2p_gz_proxy":                    p2pBlocklistProxy,
	"torproject_exits":                torprojectExits,
	"dataplane_column3":               dataplaneColumn3,
	"parse_cleantalk":                 parseCleanTalk,
	"parse_cta_cryptowall":            parseCTACryptowall,
	"parse_graphiclineweb":            parseGraphicLineWeb,
	"botscout_filter":                 botScoutFilter,
	"gz_proxyrss":                     gzProxyRSS,
	"ip2location_ip2proxy_px1lite":    ip2locationPX1Lite,
}

func Run(ctx context.Context, steps []config.ProcessorStep, input []byte) ([]byte, error) {
	started := time.Now()
	ctx, span := observability.Start(ctx, "processor.run", attribute.Int("processor.steps", len(steps)), attribute.Int("processor.input.bytes", len(input)))
	var opErr error
	defer func() {
		status := "ok"
		if opErr != nil {
			status = "error"
		}
		attrs := []attribute.KeyValue{
			attribute.String("processor.mode", "memory"),
			attribute.String("processor.status", status),
		}
		observability.TryCount("processor.runs", 1, attrs...)
		observability.TryDuration("processor.run", time.Since(started), attrs...)
		observability.End(span, opErr)
	}()
	if err := checkContext(ctx); err != nil {
		opErr = err
		return nil, err
	}
	if len(steps) == 0 {
		return input, nil
	}

	out := append([]byte(nil), input...)
	for _, step := range steps {
		if err := checkContext(ctx); err != nil {
			opErr = err
			return nil, err
		}
		name := step.Name
		if name == "" {
			name = "passthrough"
		}
		fn := registry[name]
		if fn == nil {
			opErr = fmt.Errorf("unknown processor step %q", name)
			return nil, opErr
		}
		stepStarted := time.Now()
		next, err := fn(ctx, out, step.Args)
		if err != nil {
			opErr = fmt.Errorf("%s: %w", name, err)
			observability.TryObserve("processor.step", 1, int64(len(out)), time.Since(stepStarted), attribute.String("processor.step", name), attribute.String("processor.status", "error"))
			return nil, opErr
		}
		observability.TryObserve("processor.step", 1, int64(len(out)), time.Since(stepStarted), attribute.String("processor.step", name), attribute.String("processor.status", "ok"))
		if err := checkContext(ctx); err != nil {
			opErr = err
			return nil, err
		}
		out = next
	}
	return out, nil
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

func passthrough(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return append([]byte(nil), input...), nil
}

func removeHashComments(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return normalizeCommented(input, "#"), nil
}

func removeSemiComments(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return normalizeCommented(input, ";"), nil
}

func snortRules(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(normalizeCommented(input, "#"))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.HasPrefix(line, "alert ") {
			continue
		}
		start := strings.IndexByte(line, '[')
		end := strings.IndexByte(line, ']')
		if start < 0 || end <= start {
			continue
		}
		for _, part := range strings.Split(line[start+1:end], ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return joinLines(out), nil
}

func pixDenyRules(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(normalizeCommented(input, "#"))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 7 || fields[0] != "access-list" || !strings.Contains(line, " deny ip ") {
			continue
		}
		if idx := strings.Index(line, " deny ip host "); idx >= 0 {
			rest := line[idx+len(" deny ip host "):]
			host := strings.Fields(rest)
			if len(host) > 0 {
				out = append(out, host[0])
			}
			continue
		}
		if idx := strings.Index(line, " deny ip "); idx >= 0 {
			rest := strings.Fields(line[idx+len(" deny ip "):])
			if len(rest) >= 2 {
				out = append(out, rest[0]+"/"+rest[1])
			}
		}
	}
	return joinLines(out), nil
}

func dshieldFormat(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(normalizeCommented(input, "#"))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[0] == "" || fields[0][0] < '1' || fields[0][0] > '9' {
			continue
		}
		out = append(out, fields[0]+"/"+fields[2])
	}
	return joinLines(out), nil
}

func xmlRSSTitleResolve(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	titles := extractXMLTagContent(string(input), "title")
	out := make([]string, 0, len(titles))
	for _, title := range titles {
		if idx := strings.LastIndex(title, ":"); idx > 0 {
			candidate := strings.TrimSpace(title[:idx])
			if candidate != "" {
				out = append(out, candidate)
			}
		}
	}
	return joinLines(out), nil
}

func xmlRSSProxy(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	values := extractXMLTagContent(string(input), "prx:ip")
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if net.ParseIP(value) != nil {
			out = append(out, value)
		}
	}
	return joinLines(out), nil
}

func xmlRSSTitle(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	titles := extractXMLTagContent(string(input), "title")
	out := make([]string, 0, len(titles))
	for _, title := range titles {
		if title == "" {
			continue
		}
		first := strings.Split(title, "|")[0]
		if matches := findIPv4(first); len(matches) > 0 {
			out = append(out, matches[0])
		}
	}
	return joinLines(out), nil
}

func xmlTag(_ context.Context, input []byte, args map[string]string) ([]byte, error) {
	tag := strings.TrimSpace(args["tag"])
	if tag == "" {
		tag = "ip"
	}
	return joinLines(extractXMLTagContent(string(input), tag)), nil
}

func dshieldAPIXML(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	values := extractXMLTagContent(string(input), "ip")
	out := make([]string, 0, len(values))
	for _, value := range values {
		parts := strings.Split(strings.TrimSpace(value), ".")
		if len(parts) != 4 {
			continue
		}
		fixed := make([]string, 0, 4)
		for _, part := range parts {
			n, err := strconv.Atoi(part)
			if err != nil {
				fixed = nil
				break
			}
			fixed = append(fixed, strconv.Itoa(n))
		}
		if len(fixed) == 4 {
			out = append(out, strings.Join(fixed, "."))
		}
	}
	return joinLines(out), nil
}

func subnetToCIDR(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(input)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "/")
		if len(parts) != 2 {
			out = append(out, line)
			continue
		}
		mask := net.ParseIP(strings.TrimSpace(parts[1])).To4()
		if mask == nil {
			out = append(out, line)
			continue
		}
		ones, bits := net.IPMask(mask).Size()
		if bits != 32 {
			out = append(out, line)
			continue
		}
		out = append(out, strings.TrimSpace(parts[0])+"/"+strconv.Itoa(ones))
	}
	return joinLines(out), nil
}

func extractIPv4(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return joinLines(findIPv4(string(input))), nil
}

// extractIPv4CIDR is the CIDR-preserving counterpart of
// extractIPv4. Where extract_ipv4 strips any /prefix suffix
// ("192.168.1.0/24" → "192.168.1.0"), this keeps it
// ("192.168.1.0/24" → "192.168.1.0/24") so the downstream
// iprange parser receives the full network description.
//
// Required for JSON feeds like the MISP warninglists, where
// the source publishes CIDR ranges like "103.21.244.0/22" and
// extracting only the base IP loses 99% of the covered address
// space. Safe to use on plain-IP feeds too — tokens without a
// /prefix pass through unchanged.
func extractIPv4CIDR(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return joinLines(findIPv4CIDR(string(input))), nil
}

func csvColumn(_ context.Context, input []byte, args map[string]string) ([]byte, error) {
	index := 1
	if raw := strings.TrimSpace(args["index"]); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid csv column index %q", raw)
		}
		index = n
	}
	reader := csv.NewReader(bytes.NewReader(input))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row) < index {
			continue
		}
		value := strings.TrimSpace(row[index-1])
		if value != "" {
			out = append(out, value)
		}
	}
	return joinLines(out), nil
}

func unzipFirst(_ context.Context, input []byte, args map[string]string) ([]byte, error) {
	want := strings.TrimSpace(args["file"])
	return unzipFileByName(input, want)
}

func unzipCSV(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	data, err := unzipFileByName(input, "")
	if err != nil {
		return nil, err
	}
	data = bytes.ReplaceAll(data, []byte(","), []byte("\n"))
	data = bytes.ReplaceAll(data, []byte("\r"), []byte("\n"))
	return joinLines(splitLines(data)), nil
}

func p2pBlocklist(ctx context.Context, input []byte, _ map[string]string) ([]byte, error) {
	data, err := gunzipFile(ctx, input, nil)
	if err != nil {
		return nil, err
	}
	return extractP2PRanges(data, false), nil
}

func p2pBlocklistProxy(ctx context.Context, input []byte, _ map[string]string) ([]byte, error) {
	data, err := gunzipFile(ctx, input, nil)
	if err != nil {
		return nil, err
	}
	return extractP2PRanges(data, true), nil
}

func torprojectExits(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(input)
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "ExitAddress" {
			out = append(out, fields[1])
		}
	}
	return joinLines(out), nil
}

func dataplaneColumn3(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(normalizeCommented(input, "#"))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Split(line, "|")
		if len(fields) < 3 {
			continue
		}
		value := strings.Join(strings.Fields(fields[2]), " ")
		if value != "" {
			out = append(out, value)
		}
	}
	return joinLines(out), nil
}

func parseCleanTalk(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	matches := cleanTalkPattern.FindAllStringSubmatch(string(input), -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return joinLines(out), nil
}

func parseCTACryptowall(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	reader := csv.NewReader(bytes.NewReader(input))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row) >= 3 {
			value := strings.TrimSpace(row[2])
			if value != "" {
				out = append(out, value)
			}
		}
	}
	return joinLines(out), nil
}

func parseGraphicLineWeb(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	matches := graphicLinePattern.FindAllStringSubmatch(string(input), -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		out = append(out, match[1])
	}
	return joinLines(out), nil
}

func botScoutFilter(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	matches := botScoutPattern.FindAllStringSubmatch(string(input), -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		value := strings.TrimSpace(match[1])
		if value != "" {
			out = append(out, value)
		}
	}
	return joinLines(out), nil
}

func gzProxyRSS(ctx context.Context, input []byte, _ map[string]string) ([]byte, error) {
	data, err := gunzipFile(ctx, input, nil)
	if err != nil {
		return nil, err
	}
	lines := splitLines(normalizeCommented(data, "#"))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if before, _, ok := strings.Cut(line, ":"); ok {
			before = strings.TrimSpace(before)
			if before != "" {
				out = append(out, before)
			}
		}
	}
	return joinLines(out), nil
}

func ip2locationPX1Lite(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	data, err := unzipFileByName(input, "IP2PROXY-LITE-PX1.CSV")
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		start, ok1 := numericIPv4(strings.TrimSpace(row[0]))
		end, ok2 := numericIPv4(strings.TrimSpace(row[1]))
		if !ok1 || !ok2 {
			continue
		}
		if start == end {
			out = append(out, start)
			continue
		}
		out = append(out, start+"-"+end)
	}
	return joinLines(out), nil
}
