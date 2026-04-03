package processor

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestChecklistProcessorsExecute(t *testing.T) {
	steps := []struct {
		name  string
		steps []config.ProcessorStep
		input []byte
		want  string
	}{
		{
			name:  "remove_comments_semi",
			steps: []config.ProcessorStep{{Name: "remove_comments_semi"}},
			input: []byte("1.2.3.4 ; drop\n;\n5.6.7.8\n"),
			want:  "1.2.3.4\n5.6.7.8\n",
		},
		{
			name:  "extract_ipv4",
			steps: []config.ProcessorStep{{Name: "extract_ipv4"}},
			input: []byte("allow=1.2.3.4 deny=5.6.7.8/24 ignore=999.1.1.1\n"),
			want:  "1.2.3.4\n5.6.7.8\n",
		},
		{
			name:  "csv_column",
			steps: []config.ProcessorStep{{Name: "csv_column", Args: map[string]string{"index": "2"}}},
			input: []byte("a,b,c\n1,2,3\n"),
			want:  "b\n2\n",
		},
		{
			name:  "gunzip",
			steps: []config.ProcessorStep{{Name: "gunzip"}},
			input: gzipBytes(t, "one\ntwo\n"),
			want:  "one\ntwo\n",
		},
		{
			name:  "unzip",
			steps: []config.ProcessorStep{{Name: "unzip", Args: map[string]string{"file": "data.txt"}}},
			input: zipBytes(t, "data.txt", "1.2.3.4\n"),
			want:  "1.2.3.4\n",
		},
		{
			name:  "unzip_csv",
			steps: []config.ProcessorStep{{Name: "unzip_csv"}},
			input: zipBytes(t, "data.csv", "a,b\r\n1,2\r\n"),
			want:  "a\nb\n1\n2\n",
		},
		{
			name:  "p2p_blocklist",
			steps: []config.ProcessorStep{{Name: "p2p_blocklist"}},
			input: gzipBytes(t, "Proxy:1.2.3.4-1.2.3.6\nOther:8.8.8.8-8.8.8.9\n"),
			want:  "1.2.3.4-1.2.3.6\n8.8.8.8-8.8.8.9\n",
		},
		{
			name:  "snort_rules",
			steps: []config.ProcessorStep{{Name: "snort_rules"}},
			input: []byte("alert ip any any -> any any [1.2.3.4, 5.6.7.0/24]\n"),
			want:  "1.2.3.4\n5.6.7.0/24\n",
		},
		{
			name:  "pix_deny_rules",
			steps: []config.ProcessorStep{{Name: "pix_deny_rules"}},
			input: []byte("access-list outside deny ip 1.2.3.0 255.255.255.0 any\naccess-list outside deny ip host 5.6.7.8 any\n"),
			want:  "1.2.3.0/255.255.255.0\n5.6.7.8\n",
		},
		{
			name:  "dshield_format",
			steps: []config.ProcessorStep{{Name: "dshield_format"}},
			input: []byte("1.2.3.0 x 24\n\ncomment\n"),
			want:  "1.2.3.0/24\n",
		},
		{
			name:  "xml_tag",
			steps: []config.ProcessorStep{{Name: "xml_tag", Args: map[string]string{"tag": "ip"}}},
			input: []byte("<root><ip>1.2.3.4</ip><ip>5.6.7.8</ip></root>"),
			want:  "1.2.3.4\n5.6.7.8\n",
		},
		{
			name:  "xml_rss_title",
			steps: []config.ProcessorStep{{Name: "xml_rss_title"}},
			input: []byte("<rss><title>1.2.3.4 | proxy</title><title>ignored</title></rss>"),
			want:  "1.2.3.4\n",
		},
		{
			name:  "xml_rss_proxy",
			steps: []config.ProcessorStep{{Name: "xml_rss_proxy"}},
			input: []byte("<rss><prx:ip>5.6.7.8</prx:ip></rss>"),
			want:  "5.6.7.8\n",
		},
		{
			name:  "regex",
			steps: []config.ProcessorStep{{Name: "regex", Args: map[string]string{"pattern": `ip=((?:\d{1,3}\.){3}\d{1,3})`}}},
			input: []byte("ip=1.2.3.4 ip=5.6.7.8"),
			want:  "1.2.3.4\n5.6.7.8\n",
		},
		{
			name:  "subnet_to_cidr",
			steps: []config.ProcessorStep{{Name: "subnet_to_cidr"}},
			input: []byte("1.2.3.0/255.255.255.0\n"),
			want:  "1.2.3.0/24\n",
		},
		{
			name:  "json_paths",
			steps: []config.ProcessorStep{{Name: "json_paths", Args: map[string]string{"paths": "$.a[*],$.b[*]"}}},
			input: []byte(`{"a":["1.2.3.4"],"b":["5.6.7.0/24"]}`),
			want:  "1.2.3.4\n5.6.7.0/24\n",
		},
		{
			name:  "passthrough",
			steps: []config.ProcessorStep{{Name: "passthrough"}},
			input: []byte("1.2.3.4\n"),
			want:  "1.2.3.4\n",
		},
		{
			name: "pipeline_composition",
			steps: []config.ProcessorStep{
				{Name: "remove_comments"},
				{Name: "extract_ipv4"},
				{Name: "append_slash32"},
			},
			input: []byte("allow 1.2.3.4 # keep\nallow 5.6.7.8\n"),
			want:  "1.2.3.4/32\n5.6.7.8/32\n",
		},
		{
			name:  "filter_ip4",
			steps: []config.ProcessorStep{{Name: "filter_ip4"}},
			input: []byte("1.2.3.4\n5.6.7.0/24\n2001:db8::1\n"),
			want:  "1.2.3.4\n",
		},
		{
			name:  "filter_net4",
			steps: []config.ProcessorStep{{Name: "filter_net4"}},
			input: []byte("1.2.3.4\n5.6.7.0/24\n8.8.8.8/32\n"),
			want:  "5.6.7.0/24\n",
		},
		{
			name:  "filter_all4",
			steps: []config.ProcessorStep{{Name: "filter_all4"}},
			input: []byte("1.2.3.4\n5.6.7.0/24\n2001:db8::1\n"),
			want:  "1.2.3.4\n5.6.7.0/24\n",
		},
	}

	for _, tc := range steps {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Run(t.Context(), tc.steps, tc.input)
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
			if got := string(out); got != tc.want {
				t.Fatalf("unexpected output: got %q want %q", got, tc.want)
			}
		})
	}
}

func TestChecklistProcessorsHandleEmptyMalformedAndLargeInputs(t *testing.T) {
	steps := []string{
		"remove_comments", "remove_comments_semi", "trim", "extract_ipv4", "csv_column", "cut_delimiter",
		"gunzip", "unzip", "unzip_csv", "p2p_blocklist", "snort_rules", "pix_deny_rules", "dshield_format",
		"xml_tag", "xml_rss_title", "xml_rss_proxy", "regex", "grep", "grep_not", "hostname_resolve",
		"subnet_to_cidr", "json_path", "json_paths", "passthrough", "filter_ip4", "filter_net4", "filter_all4",
		"filter_invalid4", "append_slash32", "remove_slash32",
	}

	for _, name := range steps {
		t.Run(name, func(t *testing.T) {
			step := config.ProcessorStep{Name: name, Args: checklistArgs(name)}
			for _, input := range [][]byte{nil, malformedInput(name), largeValidInput(t, name)} {
				func() {
					defer func() {
						if recovered := recover(); recovered != nil {
							t.Fatalf("processor %q panicked with input size %d: %v", name, len(input), recovered)
						}
					}()
					_, _ = Run(t.Context(), []config.ProcessorStep{step}, input)
				}()
			}
		})
	}
}

func checklistArgs(name string) map[string]string {
	switch name {
	case "csv_column":
		return map[string]string{"index": "1"}
	case "cut_delimiter":
		return map[string]string{"delimiter": "|", "field": "1"}
	case "grep", "grep_not":
		return map[string]string{"pattern": "1\\.2\\.3\\.4"}
	case "hostname_resolve":
		return map[string]string{"threads": "2"}
	case "json_path":
		return map[string]string{"path": "$.items[*].ip"}
	case "json_paths":
		return map[string]string{"paths": "$.items[*].ip,$.items[*].cidr"}
	case "regex":
		return map[string]string{"pattern": `((?:\d{1,3}\.){3}\d{1,3})`}
	case "xml_tag":
		return map[string]string{"tag": "ip"}
	case "unzip":
		return map[string]string{"file": "data.txt"}
	default:
		return nil
	}
}

func malformedInput(name string) []byte {
	switch name {
	case "gunzip", "p2p_blocklist":
		return []byte("not-gzip")
	case "unzip", "unzip_csv":
		return []byte("not-zip")
	case "json_path", "json_paths":
		return []byte(`{"items":[`)
	case "csv_column":
		return []byte(`"unterminated`)
	default:
		return []byte("malformed\ninput\n")
	}
}

func largeValidInput(t *testing.T, name string) []byte {
	t.Helper()
	switch name {
	case "gunzip", "p2p_blocklist":
		return gzipBytes(t, strings.Repeat("Proxy:1.2.3.4-1.2.3.5\n", 256))
	case "unzip":
		return zipBytes(t, "data.txt", strings.Repeat("1.2.3.4\n", 256))
	case "unzip_csv":
		return zipBytes(t, "data.csv", strings.Repeat("1.2.3.4,5.6.7.8\n", 128))
	case "xml_tag":
		return []byte("<root>" + strings.Repeat("<ip>1.2.3.4</ip>", 256) + "</root>")
	case "xml_rss_title":
		return []byte("<rss>" + strings.Repeat("<title>1.2.3.4 | proxy</title>", 256) + "</rss>")
	case "xml_rss_proxy":
		return []byte("<rss>" + strings.Repeat("<prx:ip>1.2.3.4</prx:ip>", 256) + "</rss>")
	case "json_path", "json_paths":
		items := make([]string, 0, 256)
		for i := 0; i < 256; i++ {
			items = append(items, fmt.Sprintf(`{"ip":"1.2.3.%d","cidr":"10.0.%d.0/24"}`, i%255, i%255))
		}
		return []byte(`{"items":[` + strings.Join(items, ",") + `]}`)
	case "snort_rules":
		return []byte(strings.Repeat("alert ip any any -> any any [1.2.3.4, 5.6.7.0/24]\n", 256))
	case "pix_deny_rules":
		return []byte(strings.Repeat("access-list outside deny ip 1.2.3.0 255.255.255.0 any\n", 256))
	case "dshield_format":
		return []byte(strings.Repeat("1.2.3.0 x 24\n", 256))
	case "subnet_to_cidr":
		return []byte(strings.Repeat("1.2.3.0/255.255.255.0\n", 256))
	case "hostname_resolve":
		return []byte(strings.Repeat("1.2.3.4\n", 256))
	default:
		return []byte(strings.Repeat("value 1.2.3.4\n", 256))
	}
}

func gzipBytes(t *testing.T, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipBytes(t *testing.T, name, text string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	file, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte(text)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
