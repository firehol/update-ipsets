package processor

import (
	"strings"
	"testing"

	"github.com/firehol/update-ipsets/pkg/config"
)

func TestLegacyBluelivParser(t *testing.T) {
	input := []byte(`{"crimeServers":[{"ip":"1.2.3.4"},{"ip":null},{"ip":"5.6.7.8"}]}`)
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "blueliv_parser"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4\n5.6.7.8\n"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestLegacyCleanMXPhishingCSV(t *testing.T) {
	input := []byte(`"a","b","c","d","e","f","g","h","i","1.2.3.4|note"` + "\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "parse_cvs_clean_mx_phishing"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4_note\n"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestLegacyHPHostsToIPs(t *testing.T) {
	input := []byte("127.0.0.1 localhost # local\n0.0.0.0 1.2.3.4 example.invalid\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "hphosts2ips"}}, input)
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

func TestLegacyClient9IPCatDatacenters(t *testing.T) {
	input := []byte("1.2.3.0,1.2.3.255,provider\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "parse_client9_ipcat_datacenters"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.0-1.2.3.255\n"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestLegacyIPBlacklistCloudParser(t *testing.T) {
	input := []byte(`<td>1.2.3.4</td><td>ignore</td><td>5.6.7.8</td>`)
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "parse_ipblacklistcloud"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4\n5.6.7.8\n"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestLegacyMaxmindProxyFraudParser(t *testing.T) {
	input := []byte(`
<a href="high-risk-ip-sample/1.2.3.4">1.2.3.4</a>
<a href="/en/high-risk-ip-sample/5.6.7.8"
  >5.6.7.8</a>
<a href="/en/high-risk-ip-sample/5.6.7.8">duplicate</a>
`)
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "parse_maxmind_proxy_fraud"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4\n5.6.7.8\n"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}

func TestLegacyUSCertCSV(t *testing.T) {
	input := []byte(`1.2.3.4,IP Watchlist` + "\n" + `5.6.7.8,Other` + "\n")
	out, err := Run(t.Context(), []config.ProcessorStep{{Name: "parse_uscert_csv"}}, input)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(out), "1.2.3.4\n"; got != want {
		t.Fatalf("unexpected output: got %q want %q", got, want)
	}
}
