package config

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"slices"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type ExtractOptions struct {
	// IncludeGeolocation is retained for backwards compatibility with
	// the legacy bash extractor. After source unification the option is
	// a no-op: geolocation, ASN, and bogon databases are sourced from
	// the YAML catalog rather than synthesized from the bash script.
	IncludeGeolocation bool
}

func ExtractLegacyScript(path string, opts ExtractOptions) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	_ = opts // placeholder until the legacy extractor learns about the new sources

	cfg := New()

	for _, block := range collectCommandBlocks(string(data), "update", "merge", "rename_ipset", "delete_ipset") {
		call, err := parseCommandBlock(block)
		if err != nil || len(call.Args) == 0 {
			continue
		}
		blockBytes := []byte(block)
		head := shellWordText(blockBytes, call.Args[0])
		switch head {
		case "update":
			src, err := extractSource(blockBytes, call.Args)
			if err == nil {
				cfg.Sources[src.Name] = src
			}
		case "merge":
			merge, err := extractMerge(blockBytes, call.Args)
			if err == nil {
				cfg.Merges[merge.Name] = merge
			}
		case "rename_ipset":
			args := shellWordsText(blockBytes, call.Args[1:])
			if len(args) >= 2 {
				cfg.Renames[args[0]] = args[1]
			}
		case "delete_ipset":
			args := shellWordsText(blockBytes, call.Args[1:])
			if len(args) >= 1 {
				cfg.Deleted = append(cfg.Deleted, args[0])
			}
		}
	}

	slices.Sort(cfg.Deleted)
	if err := injectBuiltInSyntheticSources(cfg); err != nil {
		return nil, err
	}
	return cfg, Validate(cfg)
}

func collectCommandBlocks(src string, commands ...string) []string {
	commandSet := map[string]struct{}{}
	for _, cmd := range commands {
		commandSet[cmd] = struct{}{}
	}

	lines := strings.Split(src, "\n")
	var blocks []string
	var current strings.Builder
	collecting := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !collecting {
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			fields := strings.Fields(trimmed)
			if len(fields) == 0 {
				continue
			}
			if _, ok := commandSet[fields[0]]; !ok {
				continue
			}
			collecting = true
			current.Reset()
			current.WriteString(trimmed)
			if strings.HasSuffix(trimmed, "\\") {
				current.WriteString("\n")
				continue
			}
			blocks = append(blocks, current.String())
			collecting = false
			continue
		}

		next := strings.TrimSpace(line)
		current.WriteString(next)
		if strings.HasSuffix(next, "\\") {
			current.WriteString("\n")
			continue
		}
		blocks = append(blocks, current.String())
		collecting = false
	}

	return blocks
}

func parseCommandBlock(block string) (*syntax.CallExpr, error) {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(block), "")
	if err != nil {
		return nil, err
	}
	if len(file.Stmts) == 0 {
		return nil, fmt.Errorf("empty command block")
	}
	call, ok := file.Stmts[0].Cmd.(*syntax.CallExpr)
	if !ok {
		return nil, fmt.Errorf("command block is not a call")
	}
	return call, nil
}

func extractSource(src []byte, args []*syntax.Word) (*Source, error) {
	fields := shellWordsText(src, args[1:])
	if len(fields) < 11 {
		return nil, fmt.Errorf("update call has %d args, want >= 11", len(fields))
	}
	if strings.Contains(fields[0], "${") || strings.Contains(fields[3], "${") {
		return nil, fmt.Errorf("dynamic helper update %q ignored", fields[0])
	}
	frequency, err := evalArithmetic(fields[1])
	if err != nil {
		return nil, fmt.Errorf("source %q frequency: %w", fields[0], err)
	}
	history, err := evalHistory(fields[2])
	if err != nil {
		return nil, fmt.Errorf("source %q history: %w", fields[0], err)
	}

	attributes, flags := extractAttributes(fields[11:])
	return &Source{
		Name:            fields[0],
		Frequency:       frequency,
		History:         history,
		IPV:             fields[3],
		Output:          normalizeOutput(fields[4]),
		URL:             fields[5],
		Processor:       normalizeProcessor(fields[6]),
		ProcessorRaw:    fields[6],
		Category:        fields[7],
		Info:            fields[8],
		Maintainer:      fields[9],
		MaintainerURL:   fields[10],
		Attributes:      attributes,
		EnabledByAll:    !flags["dont_enable_with_all"],
		Redistributable: boolPtr(!flags["dont_redistribute"]),
		AcceptEmpty:     flags["can_be_empty"] || flags["empty"],
	}, nil
}

func extractMerge(src []byte, args []*syntax.Word) (*Merge, error) {
	fields := shellWordsText(src, args[1:])
	if len(fields) < 8 {
		return nil, fmt.Errorf("merge call has %d args, want >= 8", len(fields))
	}
	if strings.Contains(fields[0], "${") || strings.Contains(fields[1], "${") {
		return nil, fmt.Errorf("dynamic helper merge %q ignored", fields[0])
	}
	return &Merge{
		Name:          fields[0],
		IPV:           fields[1],
		Output:        normalizeOutput(fields[2]),
		Category:      fields[3],
		Info:          fields[4],
		Maintainer:    fields[5],
		MaintainerURL: fields[6],
		Sources:       append([]string{}, fields[7:]...),
	}, nil
}

func shellWordsText(src []byte, words []*syntax.Word) []string {
	out := make([]string, 0, len(words))
	for _, word := range words {
		out = append(out, shellWordText(src, word))
	}
	return out
}

func shellWordText(src []byte, word *syntax.Word) string {
	var b strings.Builder
	for _, part := range word.Parts {
		b.WriteString(shellWordPartText(src, part))
	}
	return b.String()
}

func shellWordPartText(src []byte, part syntax.WordPart) string {
	switch x := part.(type) {
	case *syntax.Lit:
		return x.Value
	case *syntax.SglQuoted:
		return x.Value
	case *syntax.DblQuoted:
		var b strings.Builder
		for _, sub := range x.Parts {
			b.WriteString(shellWordPartText(src, sub))
		}
		return b.String()
	case *syntax.ParamExp, *syntax.ArithmExp, *syntax.CmdSubst:
		return rawSource(src, part.Pos(), part.End())
	default:
		return rawSource(src, part.Pos(), part.End())
	}
}

func rawSource(src []byte, pos, end syntax.Pos) string {
	start := int(pos.Offset())
	finish := int(end.Offset())
	if start < 0 || finish < start || finish > len(src) {
		return ""
	}
	return string(src[start:finish])
}

func evalHistory(raw string) ([]int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "0" {
		return nil, nil
	}
	parts := strings.Fields(raw)
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := evalArithmetic(part)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func evalArithmetic(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	if strings.HasPrefix(raw, "$[") && strings.HasSuffix(raw, "]") {
		raw = strings.TrimSuffix(strings.TrimPrefix(raw, "$["), "]")
	}
	if strings.HasPrefix(raw, "$((") && strings.HasSuffix(raw, "))") {
		raw = strings.TrimSuffix(strings.TrimPrefix(raw, "$(("), "))")
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return n, nil
	}

	expr, err := goparser.ParseExpr(raw)
	if err != nil {
		return 0, err
	}
	value, err := evalGoExpr(expr)
	if err != nil {
		return 0, err
	}
	return value, nil
}

func evalGoExpr(expr ast.Expr) (int, error) {
	switch x := expr.(type) {
	case *ast.BasicLit:
		if x.Kind != gotoken.INT {
			return 0, fmt.Errorf("unsupported literal %q", x.Value)
		}
		n, err := strconv.Atoi(x.Value)
		if err != nil {
			return 0, err
		}
		return n, nil
	case *ast.ParenExpr:
		return evalGoExpr(x.X)
	case *ast.UnaryExpr:
		value, err := evalGoExpr(x.X)
		if err != nil {
			return 0, err
		}
		switch x.Op {
		case gotoken.ADD:
			return value, nil
		case gotoken.SUB:
			return -value, nil
		default:
			return 0, fmt.Errorf("unsupported unary op %s", x.Op)
		}
	case *ast.BinaryExpr:
		left, err := evalGoExpr(x.X)
		if err != nil {
			return 0, err
		}
		right, err := evalGoExpr(x.Y)
		if err != nil {
			return 0, err
		}
		switch x.Op {
		case gotoken.ADD:
			return left + right, nil
		case gotoken.SUB:
			return left - right, nil
		case gotoken.MUL:
			return left * right, nil
		case gotoken.QUO:
			return left / right, nil
		case gotoken.REM:
			return left % right, nil
		case gotoken.SHL:
			return left << right, nil
		case gotoken.SHR:
			return left >> right, nil
		case gotoken.AND:
			return left & right, nil
		case gotoken.OR:
			return left | right, nil
		case gotoken.XOR:
			return left ^ right, nil
		default:
			return 0, fmt.Errorf("unsupported binary op %s", x.Op)
		}
	default:
		return 0, fmt.Errorf("unsupported expression %T", expr)
	}
}

func extractAttributes(fields []string) (map[string]string, map[string]bool) {
	attrs := map[string]string{}
	flags := map[string]bool{}

	for i := 0; i < len(fields); {
		key := fields[i]
		switch key {
		case "redistribute", "dont_redistribute", "can_be_empty", "empty", "never_empty", "no_empty", "no_if_modified_since", "dont_enable_with_all", "inbound", "outbound":
			flags[key] = true
			if key == "inbound" || key == "outbound" {
				attrs["protection"] = key
			}
			i++
		default:
			if i+1 >= len(fields) {
				attrs[key] = ""
				i++
				continue
			}
			attrs[key] = fields[i+1]
			i += 2
		}
	}
	return attrs, flags
}

// normalizeOutput translates the bash-era output tokens emitted
// by the legacy extractor into the canonical "ipset" / "netset"
// values the engine uses internally. It's the legacy path's
// equivalent of canonicalizeOutput for the YAML path.
func normalizeOutput(raw string) string {
	switch raw {
	case "ip", "ips":
		return "ipset"
	case "net", "nets":
		return "netset"
	case "both", "all":
		return "netset"
	default:
		return raw
	}
}

func normalizeProcessor(raw string) []ProcessorStep {
	switch raw {
	case "", "cat":
		return []ProcessorStep{{Name: "passthrough"}}
	case "remove_comments":
		return []ProcessorStep{{Name: "remove_comments"}}
	case "remove_comments_semi_colon":
		return []ProcessorStep{{Name: "remove_comments_semi"}}
	case "snort_alert_rules_to_ipv4":
		return []ProcessorStep{{Name: "snort_rules"}}
	case "pix_deny_rules_to_ipv4":
		return []ProcessorStep{{Name: "pix_deny_rules"}}
	case "dshield_parser":
		return []ProcessorStep{{Name: "dshield_format"}}
	case "parse_rss_rosinstrument":
		return []ProcessorStep{{Name: "xml_rss_title_resolve"}}
	case "parse_rss_proxy":
		return []ProcessorStep{{Name: "xml_rss_proxy"}}
	case "parse_php_rss":
		return []ProcessorStep{{Name: "xml_rss_title"}}
	case "parse_xml_clean_mx":
		return []ProcessorStep{{Name: "xml_tag", Args: map[string]string{"tag": "ip"}}}
	case "parse_dshield_api":
		return []ProcessorStep{{Name: "dshield_api_xml"}}
	case "subnet_to_bitmask":
		return []ProcessorStep{{Name: "subnet_to_cidr"}}
	case "extract_ipv4_from_any_file":
		return []ProcessorStep{{Name: "extract_ipv4"}}
	case "csv_comma_first_column":
		return []ProcessorStep{{Name: "csv_column", Args: map[string]string{"index": "1"}}}
	case "gz_remove_comments":
		return []ProcessorStep{{Name: "gunzip"}, {Name: "remove_comments"}}
	case "unzip_and_split_csv":
		return []ProcessorStep{{Name: "unzip_csv"}}
	case "unzip_and_extract":
		return []ProcessorStep{{Name: "unzip"}}
	case "p2p_gz":
		return []ProcessorStep{{Name: "p2p_blocklist"}}
	case "p2p_gz_ips":
		return []ProcessorStep{{Name: "p2p_blocklist_ips"}}
	case "p2p_gz_proxy":
		return []ProcessorStep{{Name: "p2p_blocklist_proxy"}}
	case "torproject_exits":
		return []ProcessorStep{{Name: "torproject_exits"}}
	default:
		return []ProcessorStep{{Name: raw}}
	}
}
