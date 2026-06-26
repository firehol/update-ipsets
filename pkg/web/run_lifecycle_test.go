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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
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

func TestRunServesHealthAndLightStatusWhileEngineRunBlocked(t *testing.T) {
	eng := newRunLifecycleBlockedRunEngine(t)
	addr := freeTCPAddr(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	runCtx, cancelRunServer := context.WithCancel(t.Context())
	defer cancelRunServer()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- Run(runCtx, eng, Options{
			Listen:                    addr,
			EnableAll:                 true,
			Interval:                  time.Hour,
			AdminAuthMode:             AdminAuthModeDisabled,
			AllowUnauthenticatedAdmin: true,
			Logger:                    logger,
		})
	}()

	client := &http.Client{Timeout: 2 * time.Second}
	healthURL := "http://" + addr + "/healthz"
	adminLightURL := "http://" + addr + "/api/v1/admin/status?mode=light"
	resp := waitForHTTPGet(t, client, healthURL)
	_ = resp.Body.Close()

	beforePublish := make(chan struct{})
	releasePublish := make(chan struct{})
	var releasePublishOnce sync.Once
	releaseBlockedRun := func() {
		releasePublishOnce.Do(func() { close(releasePublish) })
	}
	engineRunCtx, cancelEngineRun := context.WithCancel(t.Context())
	defer cancelEngineRun()
	engineRunDone := make(chan error, 1)
	go func() {
		_, err := eng.RunOnce(engineRunCtx, engine.RunOptions{
			Selected:   []string{"sample"},
			EnableAll:  true,
			Manual:     true,
			CleanupOld: true,
			BeforePublish: func(*engine.Report) error {
				close(beforePublish)
				select {
				case <-releasePublish:
					return nil
				case <-engineRunCtx.Done():
					return engineRunCtx.Err()
				}
			},
		})
		engineRunDone <- err
	}()
	t.Cleanup(func() {
		releaseBlockedRun()
		cancelEngineRun()
		cancelRunServer()
	})

	select {
	case <-beforePublish:
	case <-time.After(2 * time.Second):
		t.Fatal("engine run did not reach blocked before-publish hook")
	}

	body := assertHTTPStatusWithin(t, client, healthURL, http.StatusOK, time.Second)
	if string(body) != "ok\n" {
		t.Fatalf("health body while engine run blocked = %q, want ok", body)
	}

	body = assertHTTPStatusWithin(t, client, adminLightURL, http.StatusOK, time.Second)
	var lightPayload adminStatusLight
	if err := json.Unmarshal(body, &lightPayload); err != nil {
		t.Fatalf("decode admin light status while engine run blocked: %v\nbody=%s", err, body)
	}
	if !lightPayload.Engine.Running {
		t.Fatalf("admin light status running = false while engine run is blocked: %+v", lightPayload.Engine)
	}
	if lightPayload.Engine.RunState != engine.RunStateFinalizing {
		t.Fatalf("admin light status run_state = %q, want finalizing while before-publish is blocked", lightPayload.Engine.RunState)
	}
	if lightPayload.Engine.EngineLane.ActiveCount != 0 {
		t.Fatalf("admin light status engine lane active_count = %d, want 0 while finalization is blocked", lightPayload.Engine.EngineLane.ActiveCount)
	}

	releaseBlockedRun()
	select {
	case err := <-engineRunDone:
		if err != nil {
			t.Fatalf("blocked engine RunOnce returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for blocked engine run to finish after release")
	}

	cancelRunServer()
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for daemon shutdown after blocked-run liveness test")
	}
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

func TestWatchdogSelfHealthCadenceAndThreshold(t *testing.T) {
	if got, want := watchdogSelfHealthTick(2*time.Second), time.Second; got != want {
		t.Fatalf("watchdogSelfHealthTick(short) = %s, want %s", got, want)
	}
	if got, want := watchdogSelfHealthTick(2*time.Minute), 15*time.Second; got != want {
		t.Fatalf("watchdogSelfHealthTick(long) = %s, want %s", got, want)
	}
	if got, want := watchdogSelfHealthTick(40*time.Second), 10*time.Second; got != want {
		t.Fatalf("watchdogSelfHealthTick(mid) = %s, want %s", got, want)
	}
	if got, want := watchdogSelfHealthThreshold(10*time.Second, 2*time.Second), 15*time.Second; got != want {
		t.Fatalf("watchdogSelfHealthThreshold(3/2 wins) = %s, want %s", got, want)
	}
	if got, want := watchdogSelfHealthThreshold(2*time.Second, 2*time.Second), 4*time.Second; got != want {
		t.Fatalf("watchdogSelfHealthThreshold(deadline wins) = %s, want %s", got, want)
	}
}

func TestRunWatchdogContinuesAfterNotifyError(t *testing.T) {
	previousNotify := systemdWatchdogNotify
	t.Cleanup(func() { systemdWatchdogNotify = previousNotify })

	var calls atomic.Int64
	systemdWatchdogNotify = func(string) error {
		calls.Add(1)
		return errors.New("notify failed")
	}
	t.Setenv("WATCHDOG_USEC", "20000")

	ctx, cancel := context.WithCancel(t.Context())
	done := startRunWatchdog(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	waitForWebCondition(t, func() bool { return calls.Load() >= 2 }, "watchdog retry after notify error")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog loop did not stop after context cancellation")
	}
}

func TestRunWatchdogTicksWhileStartupIntegrityRecoveryBlocked(t *testing.T) {
	previousNotify := systemdWatchdogNotify
	t.Cleanup(func() { systemdWatchdogNotify = previousNotify })

	var calls atomic.Int64
	systemdWatchdogNotify = func(string) error {
		calls.Add(1)
		return nil
	}
	t.Setenv("WATCHDOG_USEC", "20000")

	startupRecoveryEntered := make(chan struct{})
	releaseStartupRecovery := make(chan struct{})
	var enterOnce sync.Once
	var releaseOnce sync.Once
	releaseStartup := func() {
		releaseOnce.Do(func() { close(releaseStartupRecovery) })
	}
	restoreHook := setStartupIntegrityRecoveryBeforeCheckHookForTest(func() {
		enterOnce.Do(func() { close(startupRecoveryEntered) })
		<-releaseStartupRecovery
	})
	t.Cleanup(restoreHook)

	eng := newRunLifecycleBlockedRunEngine(t)
	addr := freeTCPAddr(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	t.Cleanup(releaseStartup)

	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, eng, Options{
			Listen:    addr,
			EnableAll: true,
			Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		})
	}()

	select {
	case <-startupRecoveryEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("startup integrity recovery did not start")
	}
	waitForWebCondition(t, func() bool { return calls.Load() >= 2 }, "watchdog ticks while startup integrity recovery is blocked")

	releaseStartup()
	client := &http.Client{Timeout: 2 * time.Second}
	resp := waitForHTTPGet(t, client, "http://"+addr+"/healthz")
	_ = resp.Body.Close()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for daemon shutdown")
	}
}

func TestRunWatchdogUpdatesHeartbeatOnlyAfterSuccessfulNotify(t *testing.T) {
	previousNotify := systemdWatchdogNotify
	t.Cleanup(func() { systemdWatchdogNotify = previousNotify })

	var lastBeat atomic.Int64
	oldBeat := time.Now().Add(-time.Hour).UnixNano()
	lastBeat.Store(oldBeat)
	systemdWatchdogNotify = func(string) error {
		return errors.New("notify failed")
	}
	sendRunWatchdogTick(t.Context(), slog.New(slog.NewTextHandler(io.Discard, nil)), &lastBeat)
	if got := lastBeat.Load(); got != oldBeat {
		t.Fatalf("failed watchdog notify updated heartbeat to %d, want %d", got, oldBeat)
	}

	systemdWatchdogNotify = func(string) error { return nil }
	sendRunWatchdogTick(t.Context(), slog.New(slog.NewTextHandler(io.Discard, nil)), &lastBeat)
	if got := lastBeat.Load(); got <= oldBeat {
		t.Fatalf("successful watchdog notify left heartbeat at %d, want newer than %d", got, oldBeat)
	}
}

func TestStartupEntityArtifactsPanicDoesNotBlockWait(t *testing.T) {
	var logs strings.Builder
	done := startStartupEntityArtifacts(t.Context(), slog.New(slog.NewTextHandler(&logs, nil)), func(context.Context) error {
		panic("startup artifact panic")
	})

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("startup entity artifact goroutine did not close after panic")
	}
	if !strings.Contains(logs.String(), "daemon control goroutine panic recovered") {
		t.Fatalf("panic recovery log missing: %s", logs.String())
	}
}

func TestWatchdogSelfHealthStopsOnContextCancel(t *testing.T) {
	var lastBeat atomic.Int64
	lastBeat.Store(time.Now().UnixNano())
	ctx, cancel := context.WithCancel(t.Context())
	done := startWatchdogSelfHealth(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)), 20*time.Millisecond, 10*time.Millisecond, &lastBeat)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog self-health observer did not stop after context cancellation")
	}
}

func TestWatchdogDiagnosticSanitizesAndCaps(t *testing.T) {
	longPath := "/srv/update-ipsets/private/customer-a/raw/feeds/very/long/path/with/many/segments/source.ipset"
	fixture := strings.Join([]string{
		"POST /api/v1/admin/reload Authorization: Bearer abc.def.secret",
		"Cookie: session=secret-cookie",
		"password=sensitive token=abc api_key=secret-key",
		`{"token":"json-secret","password":"json-password","request_body":"1.2.3.4\n5.6.7.8"}`,
		"payload=raw-feed: 192.0.2.1 198.51.100.2/32",
		longPath,
	}, "\n")
	text := SanitizeDiagnosticText(fixture, 512)
	for _, forbidden := range []string{
		"sensitive",
		"abc.def.secret",
		"secret-cookie",
		"secret-key",
		"json-secret",
		"json-password",
		"1.2.3.4",
		"5.6.7.8",
		"192.0.2.1",
		"198.51.100.2",
		longPath,
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("diagnostic text leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, ".../many/segments/source.ipset") {
		t.Fatalf("diagnostic text did not retain bounded path suffix: %s", text)
	}
	if got := SanitizeDiagnosticText(strings.Repeat("x", 1024), 128); len(got) > 128 {
		t.Fatalf("SanitizeDiagnosticText length = %d, want capped", len(got))
	}
	if got := sanitizedGoroutineSample(1, 256); len(got) > 256 {
		t.Fatalf("sanitized goroutine sample length = %d, want capped", len(got))
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

func waitForWebCondition(t *testing.T, fn func() bool, name string) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if fn() {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", name)
		case <-ticker.C:
		}
	}
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

func assertHTTPStatusWithin(t *testing.T, client *http.Client, url string, want int, timeout time.Duration) []byte {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s did not respond within %s: %v", url, timeout, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response from %s: %v", url, err)
	}
	if resp.StatusCode != want {
		t.Fatalf("GET %s status = %d body=%q, want %d", url, resp.StatusCode, body, want)
	}
	return body
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

func newRunLifecycleBlockedRunEngine(t *testing.T) *engine.Engine {
	t.Helper()

	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
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
    url: https://example.test/list.txt
    frequency: 60
    ipv: ipv4
    output: ipset
    processor:
      - passthrough
    category: attacks
    info: sample feed
    maintainer: test
    maintainer_url: https://example.test
`, baseDir, filepath.Join(root, "history"), filepath.Join(root, "lib"), filepath.Join(root, "errors"), filepath.Join(root, "web"), filepath.Join(root, "cache"), filepath.Join(root, "tmp"))
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	eng, err := engine.New(cfgPath, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		t.Fatal(err)
	}
	// Use committed local input here. The full daemon starts scheduler staged
	// work recovery, which intentionally claims .new files before the manual
	// blocked run below can own them.
	body := filepath.Join(baseDir, "sample.ipset")
	if err := os.WriteFile(body, []byte("1.2.3.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return eng
}
