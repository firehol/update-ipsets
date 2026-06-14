package dronebl

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type ParsedBuildzone struct {
	Lists    map[string]*ListData
	Warnings []string
}

type ListData struct {
	Include *RangeSet
	Exclude *RangeSet
}

func ParseBuildzone(r io.Reader) (*ParsedBuildzone, error) {
	return parseBuildzone(r, nil)
}

func parseBuildzoneForLists(r io.Reader, lists []string) (*ParsedBuildzone, error) {
	selected := map[string]bool{"global": true}
	for _, list := range lists {
		if list != "" {
			selected[list] = true
		}
	}
	return parseBuildzone(r, selected)
}

func parseBuildzone(r io.Reader, selected map[string]bool) (*ParsedBuildzone, error) {
	parsed := &ParsedBuildzone{Lists: map[string]*ListData{}}
	current := "global"
	parsed.ensure(current)

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(strings.TrimRight(scanner.Text(), "\r"))
		if line == "" || strings.HasPrefix(line, "127.") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "$DATASET "):
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				current = fields[2]
				if shouldStoreBuildzoneList(selected, current) {
					parsed.ensure(current)
				}
			}
		case strings.HasPrefix(line, "$") || strings.HasPrefix(line, "@"):
			continue
		case strings.HasPrefix(line, ":"):
			current = listNameForClass(line)
			if shouldStoreBuildzoneList(selected, current) {
				parsed.ensure(current)
			}
		default:
			if err := parsed.parseIPLine(current, line, shouldStoreBuildzoneList(selected, current)); err != nil {
				parsed.Warnings = append(parsed.Warnings, err.Error())
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read buildzone: %w", err)
	}
	return parsed, nil
}

func shouldStoreBuildzoneList(selected map[string]bool, name string) bool {
	return selected == nil || selected[name]
}

func (p *ParsedBuildzone) ensure(name string) *ListData {
	if data, ok := p.Lists[name]; ok {
		return data
	}
	data := &ListData{
		Include: NewRangeSet(),
		Exclude: NewRangeSet(),
	}
	p.Lists[name] = data
	return data
}

func (p *ParsedBuildzone) parseIPLine(listName, line string, store bool) error {
	exclude := false
	token := line
	if strings.HasPrefix(token, "!") {
		exclude = true
		token = strings.TrimSpace(strings.TrimPrefix(token, "!"))
	}

	fields := strings.Fields(token)
	if len(fields) == 0 {
		return nil
	}
	if len(fields) > 1 && strings.HasPrefix(fields[1], ":") && isIPToken(fields[0]) {
		return nil
	}
	token = fields[0]
	if !isIPToken(token) {
		return fmt.Errorf("cannot parse line %q", line)
	}

	rng, err := parseIPToken(token)
	if err != nil {
		return fmt.Errorf("cannot parse IP token %q: %w", token, err)
	}

	if !store {
		return nil
	}
	data := p.ensure(listName)
	if exclude {
		return data.Exclude.AddRange(rng)
	}
	return data.Include.AddRange(rng)
}

func listNameForClass(line string) string {
	trimmed := strings.TrimPrefix(line, ":")
	idx := strings.Index(trimmed, ":")
	if idx >= 0 {
		trimmed = trimmed[:idx]
	}
	switch trimmed {
	case "1":
		return "tests"
	case "2":
		return "samples"
	case "3":
		return "irc_drones"
	case "5":
		return "bottler"
	case "6":
		return "unknown_worms_spambots"
	case "7":
		return "ddos_drones"
	case "8":
		return "socks_proxies"
	case "9":
		return "http_proxies"
	case "10":
		return "proxychains"
	case "11":
		return "web_page_proxies"
	case "12":
		return "open_dns_resolvers"
	case "13":
		return "bruteforce_attackers"
	case "14":
		return "wingate_proxies"
	case "15":
		return "compromised"
	case "16":
		return "autorooting_worms"
	case "17":
		return "auto_botnets"
	case "18":
		return "dns_mx_on_irc"
	case "19":
		return "abused_vpn_services"
	case "255":
		return "uncategorized"
	default:
		return "unknown"
	}
}

func isIPToken(token string) bool {
	if token == "" {
		return false
	}
	for _, ch := range token {
		if (ch >= '0' && ch <= '9') || ch == '.' || ch == '/' {
			continue
		}
		return false
	}
	return true
}

func parseIPToken(token string) (Range, error) {
	if strings.Contains(token, "/") {
		parts := strings.SplitN(token, "/", 2)
		addr, err := ParseIPv4Token(parts[0])
		if err != nil {
			return Range{}, err
		}
		prefix, err := ParsePrefix(parts[1])
		if err != nil {
			return Range{}, err
		}
		lo := Network(addr, prefix)
		return Range{Lo: lo, Hi: Broadcast(lo, prefix)}, nil
	}

	addr, err := ParseIPv4Token(token)
	if err != nil {
		return Range{}, err
	}
	return Range{Lo: addr, Hi: addr}, nil
}
