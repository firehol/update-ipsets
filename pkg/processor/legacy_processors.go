package processor

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
)

var maxmindHighRiskSamplePattern = regexp.MustCompile(`high-risk-ip-sample/((?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(?:\.(?:25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})`)

func init() {
	for name, fn := range map[string]stepFunc{
		"blueliv_parser":                  bluelivParser,
		"parse_cvs_clean_mx_phishing":     cleanMXPhishingCSV,
		"hphosts2ips":                     hpHostsToIPs,
		"parse_client9_ipcat_datacenters": client9IPCatDatacenters,
		"parse_ipblacklistcloud":          ipBlacklistCloudParser,
		"parse_maxmind_proxy_fraud":       maxmindProxyFraudParser,
		"parse_uscert_csv":                uscertCSV,
	} {
		registry[name] = fn
	}
}

func bluelivParser(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	var payload struct {
		CrimeServers []struct {
			IP *string `json:"ip"`
		} `json:"crimeServers"`
	}
	if err := json.Unmarshal(input, &payload); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(payload.CrimeServers))
	for _, item := range payload.CrimeServers {
		if item.IP == nil {
			continue
		}
		ip := strings.TrimSpace(*item.IP)
		if ip == "" || strings.EqualFold(ip, "null") {
			continue
		}
		out = append(out, ip)
	}
	return joinLines(out), nil
}

func cleanMXPhishingCSV(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	reader := csv.NewReader(bytes.NewReader(input))
	reader.FieldsPerRecord = -1
	out := make([]string, 0, 64)
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(record) < 10 {
			continue
		}
		value := strings.TrimSpace(strings.ReplaceAll(record[9], "|", "_"))
		if value != "" {
			out = append(out, value)
		}
	}
	return joinLines(out), nil
}

func hpHostsToIPs(ctx context.Context, input []byte, args map[string]string) ([]byte, error) {
	lines := splitLines(normalizeCommented(input, "#"))
	hosts := make([]string, 0, len(lines)*2)
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hosts = append(hosts, fields[1:]...)
	}
	if len(hosts) == 0 {
		return nil, nil
	}
	return hostnameResolve(ctx, joinLines(hosts), args)
}

func client9IPCatDatacenters(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	reader := csv.NewReader(bytes.NewReader(input))
	reader.FieldsPerRecord = -1
	out := make([]string, 0, 128)
	for {
		record, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(record) < 2 {
			continue
		}
		start := strings.TrimSpace(record[0])
		end := strings.TrimSpace(record[1])
		if start == "" || end == "" {
			continue
		}
		out = append(out, start+"-"+end)
	}
	return joinLines(out), nil
}

func ipBlacklistCloudParser(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	return extractTagWrappedIPv4(input), nil
}

func maxmindProxyFraudParser(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	matches := maxmindHighRiskSamplePattern.FindAllSubmatch(bytesOrWhitespace(input), -1)
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{}, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		value := strings.TrimSpace(string(match[1]))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return joinLines(out), nil
}

func uscertCSV(_ context.Context, input []byte, _ map[string]string) ([]byte, error) {
	lines := splitLines(bytesOrWhitespace(input))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.Contains(line, "IP Watchlist") {
			continue
		}
		field := line
		if comma := strings.IndexByte(line, ','); comma >= 0 {
			field = line[:comma]
		}
		field = strings.Trim(strings.TrimSpace(field), `"`)
		if field != "" {
			out = append(out, field)
		}
	}
	return joinLines(out), nil
}

func extractTagWrappedIPv4(input []byte) []byte {
	matches := cleanTalkPattern.FindAllSubmatch(input, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		ip := strings.TrimSpace(string(match[1]))
		if ip == "" {
			continue
		}
		out = append(out, ip)
	}
	return joinLines(out)
}
