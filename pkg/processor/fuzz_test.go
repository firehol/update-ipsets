package processor

import (
	"bytes"
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
)

func FuzzRunDeterministicTextProcessors(f *testing.F) {
	f.Add(0, []byte("1.2.3.4 # comment\n5.6.7.8\n"))
	f.Add(1, []byte("a,b,c\n1,2,3\n"))
	f.Add(2, []byte("alert ip any any -> any any [1.2.3.4, 5.6.7.0/24]\n"))
	f.Add(3, []byte("<root><ip>1.2.3.4</ip><title>5.6.7.8 | proxy</title></root>"))

	f.Fuzz(func(t *testing.T, selector int, data []byte) {
		if len(data) > 1<<20 {
			t.Skip("input exceeds bounded processor fuzz size")
		}
		steps := deterministicProcessorFuzzSteps(selector)
		first, firstErr := Run(t.Context(), steps, data)
		second, secondErr := Run(t.Context(), steps, data)
		if (firstErr == nil) != (secondErr == nil) {
			t.Fatalf("processor determinism error mismatch: first=%v second=%v", firstErr, secondErr)
		}
		if firstErr == nil && !bytes.Equal(first, second) {
			t.Fatalf("processor determinism output mismatch:\nfirst=%q\nsecond=%q", first, second)
		}
	})
}

func deterministicProcessorFuzzSteps(selector int) []config.ProcessorStep {
	steps := [][]config.ProcessorStep{
		{{Name: "remove_comments"}},
		{{Name: "remove_comments_semi"}},
		{{Name: "trim"}},
		{{Name: "extract_ipv4"}},
		{{Name: "extract_ipv4_cidr"}},
		{{Name: "csv_column", Args: map[string]string{"index": "1"}}},
		{{Name: "cut_delimiter", Args: map[string]string{"delimiter": "|", "field": "1"}}},
		{{Name: "subnet_to_cidr"}},
		{{Name: "append_slash32"}},
		{{Name: "remove_slash32"}},
		{{Name: "filter_ip4"}},
		{{Name: "filter_net4"}},
		{{Name: "filter_all4"}},
		{{Name: "filter_invalid4"}},
		{{Name: "grep", Args: map[string]string{"pattern": "1\\.2\\.3\\.4"}}},
		{{Name: "grep_not", Args: map[string]string{"pattern": "1\\.2\\.3\\.4"}}},
		{{Name: "regex", Args: map[string]string{"pattern": `((?:\d{1,3}\.){3}\d{1,3})`}}},
		{{Name: "xml_tag", Args: map[string]string{"tag": "ip"}}},
		{{Name: "xml_rss_title"}},
		{{Name: "xml_rss_proxy"}},
		{{Name: "dshield_format"}},
		{{Name: "snort_rules"}},
		{{Name: "pix_deny_rules"}},
		{
			{Name: "remove_comments"},
			{Name: "extract_ipv4"},
			{Name: "append_slash32"},
		},
	}
	idx := selector % len(steps)
	if idx < 0 {
		idx += len(steps)
	}
	return steps[idx]
}
