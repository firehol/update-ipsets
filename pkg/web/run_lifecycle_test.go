package web

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/engine"
)

func TestRunServesHTTPS(t *testing.T) {
	eng, _ := testHandler(t, Options{EnableAll: true})
	certPath, keyPath := writeTestCertificate(t)

	addr := freeTCPAddr(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, eng, Options{
			Listen:    addr,
			EnableAll: true,
			CertFile:  certPath,
			KeyFile:   keyPath,
		})
	}()

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 2 * time.Second,
	}

	resp := waitForHTTPGet(t, client, "https://"+addr+"/healthz")
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "ok\n" {
		t.Fatalf("unexpected https response: status=%d body=%q", resp.StatusCode, body)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for https daemon shutdown")
	}
	assertHTTPServerClosed(t, client, "https://"+addr+"/healthz")
}

func TestRunServesSplitAdminOnSeparateListeners(t *testing.T) {
	t.Setenv("UPDATE_IPSETS_ADMIN_USER", "admin")
	t.Setenv("UPDATE_IPSETS_ADMIN_PASSWORD", "secret")

	eng, _ := testHandlerWithRuntime(
		t,
		Options{EnableAll: true},
		`  public_base_url: "https://public.example.test/base"
`,
	)
	publicAddr := freeTCPAddr(t)
	adminAddr := freeTCPAddr(t)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, eng, Options{
			Listen:      publicAddr,
			AdminListen: adminAddr,
			EnableAll:   true,
		})
	}()

	client := &http.Client{Timeout: 2 * time.Second}

	resp := waitForHTTPGet(t, client, "http://"+publicAddr+"/healthz")
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || string(body) != "ok\n" {
		t.Fatalf("unexpected public health response: status=%d body=%q", resp.StatusCode, body)
	}

	resp, err = client.Get("http://" + publicAddr + "/admin")
	if err != nil {
		t.Fatalf("public /admin request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("public /admin status = %d, want 404", resp.StatusCode)
	}

	resp, err = client.Get("http://" + adminAddr + "/healthz")
	if err != nil {
		t.Fatalf("admin /healthz request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("admin /healthz status = %d, want 404", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, "http://"+adminAddr+"/admin", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("admin", "secret")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("admin UI request failed: %v", err)
	}
	body, err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin UI status = %d, want 200", resp.StatusCode)
	}
	if !strings.Contains(string(body), "/static/assets/") {
		t.Fatalf("admin UI body missing embedded assets: %s", body)
	}

	req, err = http.NewRequest(http.MethodGet, "http://"+adminAddr+"/api/v1/admin/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.SetBasicAuth("admin", "secret")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("admin status request failed: %v", err)
	}
	body, err = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin status code = %d, want 200", resp.StatusCode)
	}
	var statusPayload adminStatus
	if err := json.Unmarshal(body, &statusPayload); err != nil {
		t.Fatalf("decode admin status response: %v\nbody=%s", err, body)
	}
	if got, want := statusPayload.PublicBaseURL, "https://public.example.test/base"; got != want {
		t.Fatalf("admin status public_base_url = %q, want %q", got, want)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for split daemon shutdown")
	}
	assertHTTPServerClosed(t, client, "http://"+publicAddr+"/healthz")
	assertHTTPServerClosed(t, client, "http://"+adminAddr+"/admin")
}

func TestRunRejectsSplitAdminWithoutPublicBaseURL(t *testing.T) {
	eng, _ := testHandler(t, Options{EnableAll: true})
	err := Run(t.Context(), eng, Options{
		Listen:      "127.0.0.1:18888",
		AdminListen: "127.0.0.1:18889",
		EnableAll:   true,
	})
	if err == nil || !strings.Contains(err.Error(), "runtime.public_base_url") {
		t.Fatalf("expected public_base_url validation error, got %v", err)
	}
}

func TestRunRejectsDisabledAdminWithoutAcknowledgement(t *testing.T) {
	eng, _ := testHandler(t, Options{EnableAll: true})
	err := Run(t.Context(), eng, Options{
		Listen:        "127.0.0.1:18888",
		EnableAll:     true,
		AdminAuthMode: AdminAuthModeDisabled,
	})
	if err == nil || !strings.Contains(err.Error(), "--allow-unauthenticated-admin") {
		t.Fatalf("expected unauthenticated-admin acknowledgement error, got %v", err)
	}
}

func TestPrepareEngineForRunCleansOnlyOldPublishStages(t *testing.T) {
	root := t.TempDir()
	cfgPath := filepath.Join(root, "config.yaml")
	cfg := fmt.Sprintf(`
runtime:
  base_dir: %q
  history_dir: %q
  lib_dir: %q
  errors_dir: %q
  web_dir: %q
  cache_dir: %q
  tmp_dir: %q
  ipsets_apply: false
sources:
  sample:
    static:
      - 10.0.0.1
    frequency: 0
    ipv: ipv4
    output: netset
    processor:
      - passthrough
`, filepath.Join(root, "base"), filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), filepath.Join(root, "tmp"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}

	oldWebStage := filepath.Join(eng.Runtime().WebDir, ".update-ipsets-web-old")
	recentWebStage := filepath.Join(eng.Runtime().WebDir, ".update-ipsets-web-recent")
	oldEntityStage := filepath.Join(eng.Runtime().LibDir, "entities", ".update-ipsets-entities-old")
	recentEntityStage := filepath.Join(eng.Runtime().LibDir, "entities", ".update-ipsets-entities-recent")
	for _, dir := range []string{oldWebStage, recentWebStage, oldEntityStage, recentEntityStage} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", dir, err)
		}
	}
	oldTime := time.Now().Add(-10 * time.Minute)
	recentTime := time.Now().Add(-time.Minute)
	for _, dir := range []string{oldWebStage, oldEntityStage} {
		if err := os.Chtimes(dir, oldTime, oldTime); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", dir, err)
		}
	}
	for _, dir := range []string{recentWebStage, recentEntityStage} {
		if err := os.Chtimes(dir, recentTime, recentTime); err != nil {
			t.Fatalf("Chtimes(%q) error = %v", dir, err)
		}
	}

	if err := prepareEngineForRun(eng, Options{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{oldWebStage, oldEntityStage} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected old stage %q to be removed, stat err = %v", path, err)
		}
	}
	for _, path := range []string{recentWebStage, recentEntityStage} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected recent stage %q to remain, stat err = %v", path, err)
		}
	}
}

func TestDelayedPublishStageCleanupStopsOnContextCancel(t *testing.T) {
	eng, _ := testHandler(t, Options{EnableAll: true})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	done := startDelayedPublishStageCleanup(ctx, eng, Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}, time.Now().UTC())

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("delayed publish-stage cleanup did not stop after context cancellation")
	}
}

func writeTestCertificate(t *testing.T) (string, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "127.0.0.1",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

func freeTCPAddr(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func waitForHTTPGet(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()

	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		resp, err := client.Get(url)
		if err == nil {
			return resp
		}
		lastErr = err
		select {
		case <-deadline.C:
			t.Fatalf("GET %s failed before deadline: %v", url, lastErr)
		case <-ticker.C:
		}
	}
}

func assertHTTPServerClosed(t *testing.T, client *http.Client, url string) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()
	t.Fatalf("expected %s to be closed after Run returned, got status %d", url, resp.StatusCode)
}
