package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
)

func init() {
	for name, fn := range map[string]stepFunc{
		"trim":              trimLines,
		"grep":              grepLines,
		"grep_not":          grepNotLines,
		"cut_delimiter":     cutDelimiter,
		"hostname_resolve":  hostnameResolve,
		"hostname_resolver": hostnameResolve,
		"json_path":         jsonPath,
		"json_paths":        jsonPaths,
		"remove_slash32":    removeSlash32,
		"append_slash32":    appendSlash32,
		"remove_slash128":   removeSlash128,
		"append_slash128":   appendSlash128,
		"filter_ip4":        filterIP4,
		"filter_net4":       filterNet4,
		"filter_all4":       filterAll4,
		"filter_invalid4":   filterInvalid4,
		"filter_ip6":        filterIP6,
		"filter_net6":       filterNet6,
		"filter_all6":       filterAll6,
	} {
		registry[name] = fn
	}
}

func trimLines(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return normalizeWhitespace(input), nil
}

func grepLines(_ context.Context, input []byte, args map[string]string) ([]byte, error) {
	return grepCommon(input, args, false)
}

func grepNotLines(_ context.Context, input []byte, args map[string]string) ([]byte, error) {
	return grepCommon(input, args, true)
}

func grepCommon(input []byte, args map[string]string, invert bool) ([]byte, error) {
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
	lines := splitLines(bytesOrWhitespace(input))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		matched := re.MatchString(line)
		if invert {
			matched = !matched
		}
		if matched {
			out = append(out, line)
		}
	}
	return joinLines(out), nil
}

func cutDelimiter(_ context.Context, input []byte, args map[string]string) ([]byte, error) {
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
	lines := splitLines(bytesOrWhitespace(input))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, delimiter)
		if field > len(parts) {
			continue
		}
		value := strings.TrimSpace(parts[field-1])
		if value != "" {
			out = append(out, value)
		}
	}
	return joinLines(out), nil
}

func hostnameResolve(ctx context.Context, input []byte, args map[string]string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	lines := splitLines(bytesOrWhitespace(input))
	if len(lines) == 0 {
		return nil, nil
	}
	workers := 10
	if raw := strings.TrimSpace(args["threads"]); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			workers = n
		}
	}
	if workers > 100 {
		workers = 100
	}
	type job struct {
		index int
		line  string
	}
	results := make([][]string, len(lines))
	jobs := make(chan job)
	var wg sync.WaitGroup
	var firstErr error
	var errMu sync.Mutex

	resolveLine := func(line string) ([]string, error) {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
			return nil, nil
		case net.ParseIP(line) != nil:
			return []string{line}, nil
		case strings.Contains(line, "/") || strings.Contains(line, "-"):
			return []string{line}, nil
		default:
			addrs, err := net.DefaultResolver.LookupIPAddr(ctx, line)
			if err != nil {
				return nil, nil
			}
			out := make([]string, 0, len(addrs))
			for _, addr := range addrs {
				if ip := addr.IP.To4(); ip != nil {
					out = append(out, ip.String())
				}
			}
			return out, nil
		}
	}

	worker := func() {
		for item := range jobs {
			values, err := resolveLine(item.line)
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				errMu.Unlock()
				continue
			}
			results[item.index] = values
		}
	}
	for i := 0; i < workers; i++ {
		wg.Go(worker)
	}
	for i, line := range lines {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return nil, ctx.Err()
		case jobs <- job{index: i, line: line}:
		}
	}
	close(jobs)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if firstErr != nil {
		return nil, firstErr
	}
	out := make([]string, 0, len(lines))
	for _, values := range results {
		out = append(out, values...)
	}
	return joinLines(out), nil
}

func jsonPath(_ context.Context, input []byte, args map[string]string) ([]byte, error) {
	path := strings.TrimSpace(args["path"])
	if path == "" {
		path = strings.TrimSpace(args["value"])
	}
	if path == "" {
		return nil, fmt.Errorf("missing json path")
	}
	var decoded any
	if err := json.Unmarshal(input, &decoded); err != nil {
		return nil, err
	}
	values, err := jsonLookup(decoded, parseJSONPath(path))
	if err != nil {
		return nil, err
	}
	return joinLines(jsonValuesToLines(values)), nil
}

func jsonPaths(_ context.Context, input []byte, args map[string]string) ([]byte, error) {
	raw := strings.TrimSpace(args["paths"])
	if raw == "" {
		raw = strings.TrimSpace(args["path"])
	}
	if raw == "" {
		raw = strings.TrimSpace(args["value"])
	}
	if raw == "" {
		return nil, fmt.Errorf("missing json paths")
	}
	paths := splitJSONPaths(raw)
	if len(paths) == 0 {
		return nil, fmt.Errorf("missing json paths")
	}
	var decoded any
	if err := json.Unmarshal(input, &decoded); err != nil {
		return nil, err
	}
	out := make([]string, 0)
	for _, path := range paths {
		values, err := jsonLookup(decoded, parseJSONPath(path))
		if err != nil {
			return nil, err
		}
		out = append(out, jsonValuesToLines(values)...)
	}
	return joinLines(out), nil
}

func removeSlash32(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return rewriteSuffix(input, "/32", ""), nil
}

func appendSlash32(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return appendSlash(input, "/32"), nil
}

func removeSlash128(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return rewriteSuffix(input, "/128", ""), nil
}

func appendSlash128(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return appendSlash(input, "/128"), nil
}

func filterInvalid4(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(bytesOrWhitespace(input))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "0.0.0.0" || strings.HasSuffix(line, "/0") {
			continue
		}
		out = append(out, line)
	}
	return joinLines(out), nil
}

func filterIP4(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(stringOrNil(rewriteSuffix(input, "/32", "")))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && net.ParseIP(line).To4() != nil {
			out = append(out, line)
		}
	}
	return joinLines(out), nil
}

func filterNet4(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(stringOrNil(rewriteSuffix(input, "/32", "")))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if _, _, err := net.ParseCIDR(line); err == nil {
			if !strings.HasSuffix(line, "/32") && !strings.HasSuffix(line, "/0") {
				out = append(out, line)
			}
		}
	}
	return joinLines(out), nil
}

func filterAll4(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(bytesOrWhitespace(input))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case net.ParseIP(line).To4() != nil:
			out = append(out, line)
		case isCIDRv4(line):
			out = append(out, line)
		}
	}
	return joinLines(out), nil
}

func filterIP6(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(stringOrNil(rewriteSuffix(input, "/128", "")))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if ip := net.ParseIP(line); ip != nil && ip.To4() == nil {
			out = append(out, line)
		}
	}
	return joinLines(out), nil
}

func filterNet6(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(stringOrNil(rewriteSuffix(input, "/128", "")))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if _, network, err := net.ParseCIDR(line); err == nil && network.IP.To4() == nil {
			out = append(out, line)
		}
	}
	return joinLines(out), nil
}

func filterAll6(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(bytesOrWhitespace(input))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case isIPv6(line):
			out = append(out, line)
		case isCIDRv6(line):
			out = append(out, line)
		}
	}
	return joinLines(out), nil
}

func normalizeWhitespace(input []byte) []byte {
	lines := splitLines(bytesOrWhitespace(input))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	return joinLines(out)
}

func rewriteSuffix(input []byte, from, to string) []byte {
	lines := splitLines(bytesOrWhitespace(input))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, from) {
			line = strings.TrimSuffix(line, from) + to
		}
		if line != "" {
			out = append(out, line)
		}
	}
	return joinLines(out)
}

func appendSlash(input []byte, suffix string) []byte {
	lines := splitLines(bytesOrWhitespace(input))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		value := parts[0]
		if !strings.Contains(value, "/") {
			value += suffix
		}
		out = append(out, value)
	}
	return joinLines(out)
}

func parseJSONPath(path string) []string {
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")
	path = strings.ReplaceAll(path, "[*]", ".*")
	path = strings.ReplaceAll(path, "]", "")
	path = strings.ReplaceAll(path, "[", ".#")
	parts := strings.Split(path, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func splitJSONPaths(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t'
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			out = append(out, field)
		}
	}
	return out
}

func jsonValuesToLines(values []any) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		switch x := value.(type) {
		case nil:
		case string:
			if x != "" {
				out = append(out, x)
			}
		case float64:
			out = append(out, strconv.FormatFloat(x, 'f', -1, 64))
		case bool:
			out = append(out, strconv.FormatBool(x))
		default:
			data, err := json.Marshal(x)
			if err == nil && len(data) > 0 {
				out = append(out, string(data))
			}
		}
	}
	return out
}

func jsonLookup(node any, path []string) ([]any, error) {
	if len(path) == 0 {
		return []any{node}, nil
	}
	segment := path[0]
	rest := path[1:]
	switch x := node.(type) {
	case map[string]any:
		if segment == "*" {
			keys := make([]string, 0, len(x))
			for key := range x {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			out := make([]any, 0, len(keys))
			for _, key := range keys {
				next, err := jsonLookup(x[key], rest)
				if err != nil {
					return nil, err
				}
				out = append(out, next...)
			}
			return out, nil
		}
		value, ok := x[segment]
		if !ok {
			return nil, nil
		}
		return jsonLookup(value, rest)
	case []any:
		if segment == "*" {
			out := make([]any, 0, len(x))
			for _, value := range x {
				next, err := jsonLookup(value, rest)
				if err != nil {
					return nil, err
				}
				out = append(out, next...)
			}
			return out, nil
		}
		if !strings.HasPrefix(segment, "#") {
			return nil, fmt.Errorf("array path segment %q must use [n] or [*]", segment)
		}
		index, err := strconv.Atoi(strings.TrimPrefix(segment, "#"))
		if err != nil || index < 0 || index >= len(x) {
			return nil, nil
		}
		return jsonLookup(x[index], rest)
	default:
		return nil, nil
	}
}

func bytesOrWhitespace(input []byte) []byte {
	if len(input) == 0 {
		return nil
	}
	return input
}

func stringOrNil(input []byte) []byte {
	return input
}

func isCIDRv4(line string) bool {
	ip, _, err := net.ParseCIDR(line)
	return err == nil && ip.To4() != nil
}

func isCIDRv6(line string) bool {
	ip, _, err := net.ParseCIDR(line)
	return err == nil && ip.To4() == nil
}

func isIPv6(line string) bool {
	ip := net.ParseIP(line)
	return ip != nil && ip.To4() == nil
}
