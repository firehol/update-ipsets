package processor

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
)

func loadFireholCatalog(t *testing.T) *config.Config {
	t.Helper()
	path := filepath.Join("..", "..", "configs", "firehol")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("firehol catalog not available: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestRemoveComments(t *testing.T) {
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "remove_comments"}}, []byte("1.2.3.4 # comment\n\n5.6.7.8\t\t# test\n"))
	if err != nil {
		t.Fatal(err)
	}
	want := "1.2.3.4\n5.6.7.8\n"
	if string(out) != want {
		t.Fatalf("unexpected output:\n%s\nwant:\n%s", out, want)
	}
}

func TestP2PBlocklist(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte("Proxy:1.2.3.4-1.2.3.6\nOther:8.8.8.8-8.8.8.9\n"))
	_ = zw.Close()

	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "p2p_blocklist_proxy"}}, gz.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4-1.2.3.6\n"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestGunzipFileHonorsCanceledContext(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write([]byte("1.2.3.4\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := gunzipFile(ctx, gz.Bytes(), nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("gunzipFile err=%v, want context.Canceled", err)
	}
}

func TestRunHonorsCanceledContextBeforeProcessing(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := Run(ctx, []config.ProcessorStep{{Name: "passthrough"}}, []byte("1.2.3.4\n"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run err=%v, want context.Canceled", err)
	}
}

func TestRunStopsBetweenByteStepsAfterContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	firstName := "test_cancel_after_first"
	secondName := "test_must_not_run_after_cancel"

	oldFirst, hadFirst := registry[firstName]
	oldSecond, hadSecond := registry[secondName]
	t.Cleanup(func() {
		if hadFirst {
			registry[firstName] = oldFirst
		} else {
			delete(registry, firstName)
		}
		if hadSecond {
			registry[secondName] = oldSecond
		} else {
			delete(registry, secondName)
		}
		cancel()
	})

	firstCalled := false
	secondCalled := false
	registry[firstName] = func(context.Context, []byte, map[string]string) ([]byte, error) {
		firstCalled = true
		cancel()
		return []byte("changed\n"), nil
	}
	registry[secondName] = func(context.Context, []byte, map[string]string) ([]byte, error) {
		secondCalled = true
		return []byte("unexpected\n"), nil
	}

	_, err := Run(ctx, []config.ProcessorStep{{Name: firstName}, {Name: secondName}}, []byte("1.2.3.4\n"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run err=%v, want context.Canceled", err)
	}
	if !firstCalled {
		t.Fatal("first processor was not called")
	}
	if secondCalled {
		t.Fatal("second processor ran after context cancellation")
	}
}

func TestIP2LocationPX1Lite(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	file, err := zw.Create("IP2PROXY-LITE-PX1.CSV")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = file.Write([]byte("\"16777216\",\"16777217\"\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "ip2location_ip2proxy_px1lite"}}, archive.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.0.0.0-1.0.0.1\n"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestTrim(t *testing.T) {
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "trim"}}, []byte("  a\t\tb  \n\n c \n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "a b\nc\n"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestCutDelimiter(t *testing.T) {
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "cut_delimiter", Args: map[string]string{"delimiter": "|", "field": "3"}}}, []byte("a|b| c \n1|2\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "c\n"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestGrepAndGrepNot(t *testing.T) {
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "grep", Args: map[string]string{"pattern": "^keep"}}}, []byte("keep-one\ndrop\nkeep-two\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "keep-one\nkeep-two\n"; got != want {
		t.Fatalf("unexpected grep output: got %q want %q", got, want)
	}

	out, err = Run(t.Context(), []config.ProcessorStep{{Name: "grep_not", Args: map[string]string{"pattern": "^keep"}}}, []byte("keep-one\ndrop\nkeep-two\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "drop\n"; got != want {
		t.Fatalf("unexpected grep_not output: got %q want %q", got, want)
	}
}

func TestHostnameResolve(t *testing.T) {
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "hostname_resolve"}}, []byte("localhost\n1.2.3.4\n"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if !strings.Contains(got, "127.0.0.1") {
		t.Fatalf("expected localhost resolution in %q", got)
	}
	if !strings.Contains(got, "1.2.3.4") {
		t.Fatalf("expected passthrough IP in %q", got)
	}
}

func TestJSONPath(t *testing.T) {
	input := []byte(`{"items":[{"ip":"1.2.3.4"},{"ip":"5.6.7.8"}]}`)
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "json_path", Args: map[string]string{"path": "$.items[*].ip"}}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4\n5.6.7.8\n"; got != want {
		t.Fatalf("unexpected json_path output: got %q want %q", got, want)
	}
}

func TestJSONPaths(t *testing.T) {
	input := []byte(`{"core":["1.2.3.4","5.6.7.8"],"jobs":["9.9.9.9"],"nested":{"api":["10.0.0.0/24"]}}`)
	out, err := Run(t.Context(), []config.ProcessorStep{{
		Name: "json_paths",
		Args: map[string]string{"paths": "$.core[*], $.nested.api[*]"},
	}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4\n5.6.7.8\n10.0.0.0/24\n"; got != want {
		t.Fatalf("unexpected json_paths output: got %q want %q", got, want)
	}
}

func TestIPv4Filters(t *testing.T) {
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "filter_invalid4"}}, []byte("0.0.0.0\n1.2.3.4\n5.6.7.0/0\n8.8.8.8\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4\n8.8.8.8\n"; got != want {
		t.Fatalf("unexpected filter_invalid4 output: got %q want %q", got, want)
	}

	out, err = Run(t.Context(), []config.ProcessorStep{{Name: "append_slash32"}}, []byte("1.2.3.4\n5.6.7.0/24\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4/32\n5.6.7.0/24\n"; got != want {
		t.Fatalf("unexpected append_slash32 output: got %q want %q", got, want)
	}

	out, err = Run(t.Context(), []config.ProcessorStep{{Name: "remove_slash32"}}, []byte("1.2.3.4/32\n5.6.7.0/24\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4\n5.6.7.0/24\n"; got != want {
		t.Fatalf("unexpected remove_slash32 output: got %q want %q", got, want)
	}
}

func TestFireholCatalogProcessorsAreRegistered(t *testing.T) {
	cfg := loadFireholCatalog(t)
	// Post-ExpandDerivatives plus the hidden geo synthetic feeds now
	// expands the catalog to 423 runtime sources.
	if got, want := len(cfg.Sources), 423; got != want {
		t.Fatalf("unexpected source count: got %d want %d", got, want)
	}
	if got, want := len(cfg.SourcesWithUse(config.UseGeoIP)), 5; got != want {
		t.Fatalf("unexpected geoip source count: got %d want %d", got, want)
	}

	for name, src := range cfg.Sources {
		for _, step := range src.Processor {
			if _, ok := registry[step.Name]; !ok {
				t.Fatalf("source %q references unknown processor %q", name, step.Name)
			}
		}
	}
}

func TestCriticalServiceCatalogProcessorsExtractRepresentativePayloads(t *testing.T) {
	cfg := loadFireholCatalog(t)
	tests := []struct {
		name   string
		source string
		input  string
		want   string
	}{
		{
			name:   "github core services exclude hosted compute buckets",
			source: "critical_soft_github_services",
			input: `{
				"hooks":["1.1.1.0/24"],
				"web":["2.2.2.0/24"],
				"api":["3.3.3.0/24"],
				"git":["4.4.4.0/24"],
				"packages":["5.5.5.0/24"],
				"pages":["6.6.6.0/24"],
				"copilot":["7.7.7.0/24"],
				"importer":["8.8.8.0/24"],
				"github_enterprise_importer":["9.9.9.0/24"],
				"actions":["10.10.10.0/24"],
				"actions_macos":["11.11.11.0/24"],
				"codespaces":["12.12.12.0/24"]
			}`,
			want: "1.1.1.0/24\n2.2.2.0/24\n3.3.3.0/24\n4.4.4.0/24\n5.5.5.0/24\n6.6.6.0/24\n7.7.7.0/24\n8.8.8.0/24\n9.9.9.0/24\n",
		},
		{
			name:   "github hosted compute is contextual",
			source: "critical_context_github_hosted_compute",
			input: `{
				"actions":["10.10.10.0/24"],
				"actions_macos":["11.11.11.0/24"],
				"codespaces":["12.12.12.0/24"],
				"web":["2.2.2.0/24"]
			}`,
			want: "10.10.10.0/24\n11.11.11.0/24\n12.12.12.0/24\n",
		},
		{
			name:   "salesforce hyperforce array field",
			source: "critical_soft_salesforce_hyperforce",
			input:  `{"prefixes":[{"ip_prefix":["13.108.0.0/14"]},{"ip_prefix":["66.231.80.0/20"]}]}`,
			want:   "13.108.0.0/14\n66.231.80.0/20\n",
		},
		{
			name:   "microsoft 365 endpoint array",
			source: "critical_soft_microsoft365",
			input:  `[{"serviceArea":"Exchange","ips":["40.92.0.0/15","40.107.0.0/16"]}]`,
			want:   "40.92.0.0/15\n40.107.0.0/16\n",
		},
		{
			name:   "braintree mixes cidrs and addresses",
			source: "critical_soft_braintree",
			input: `{
				"production":{"cidrs":["64.4.240.0/21"],"ips":["173.0.84.225"],"outboundIps":["173.0.92.1"]},
				"sandbox":{"cidrs":["64.4.248.0/21"],"ips":["173.0.82.83"],"outboundIps":["173.0.81.1"]}
			}`,
			want: "64.4.240.0/21\n173.0.84.225/32\n173.0.92.1/32\n64.4.248.0/21\n173.0.82.83/32\n173.0.81.1/32\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := cfg.Sources[tc.source]
			if src == nil {
				t.Fatalf("missing catalog source %q", tc.source)
			}
			out, err := Run(t.Context(), src.Processor, []byte(tc.input))
			if err != nil {
				t.Fatalf("processor for %s failed: %v", tc.source, err)
			}
			if got := string(out); got != tc.want {
				t.Fatalf("processor for %s produced %q, want %q", tc.source, got, tc.want)
			}
		})
	}
}

// TestFireholCatalogProcessorRawNamesAreRegistered verifies that the
// original bash processor names (stored in processor_raw) are also
// registered in the Go registry, ensuring backward compatibility.
func TestFireholCatalogProcessorRawNamesAreRegistered(t *testing.T) {
	cfg := loadFireholCatalog(t)

	for name, src := range cfg.Sources {
		raw := strings.TrimSpace(src.ProcessorRaw)
		if raw == "" {
			continue
		}
		if _, ok := registry[raw]; !ok {
			t.Errorf("source %q processor_raw %q is not registered in the Go processor registry",
				name, raw)
		}
	}
}

// TestFireholCatalogProcessorRawAndProcessorConsistent verifies that for
// each source, the normalized processor list and the raw processor name
// refer to the same underlying function.
func TestFireholCatalogProcessorRawAndProcessorConsistent(t *testing.T) {
	cfg := loadFireholCatalog(t)

	for name, src := range cfg.Sources {
		raw := strings.TrimSpace(src.ProcessorRaw)
		if raw == "" || len(src.Processor) == 0 {
			continue
		}

		// The raw name must be registered.
		rawFn, rawOK := registry[raw]
		if !rawOK {
			t.Errorf("source %q: processor_raw %q not registered", name, raw)
			continue
		}

		// For simple (single-step, no-args) processors, the normalized name
		// and raw name must resolve to the same function.
		if len(src.Processor) == 1 && len(src.Processor[0].Args) == 0 {
			normFn, normOK := registry[src.Processor[0].Name]
			if !normOK {
				t.Errorf("source %q: processor %q not registered", name, src.Processor[0].Name)
				continue
			}
			// Compare function pointers via fmt to check they are the same function.
			rawPtr := fmt.Sprintf("%v", rawFn)
			normPtr := fmt.Sprintf("%v", normFn)
			if rawPtr != normPtr {
				t.Errorf("source %q: processor_raw %q and processor %q resolve to different functions",
					name, raw, src.Processor[0].Name)
			}
		}
	}
}
