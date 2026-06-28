package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

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
	done := startRunWatchdog(ctx)
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
	startupServingProbe := make(chan error, 1)
	var enterOnce sync.Once
	var releaseOnce sync.Once
	releaseStartup := func() {
		releaseOnce.Do(func() { close(releaseStartupRecovery) })
	}

	eng := newRunLifecycleBlockedRunEngine(t)
	addr := freeTCPAddr(t)
	client := &http.Client{Timeout: time.Second}
	restoreHook := setStartupIntegrityRecoveryBeforeCheckHookForTest(func() {
		enterOnce.Do(func() {
			resp, err := client.Get("http://" + addr + "/healthz")
			if err != nil {
				startupServingProbe <- err
			} else {
				_ = resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					startupServingProbe <- fmt.Errorf("startup health status = %d, want 200", resp.StatusCode)
				} else {
					startupServingProbe <- nil
				}
			}
			close(startupRecoveryEntered)
		})
		<-releaseStartupRecovery
	})
	t.Cleanup(restoreHook)

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
	if err := <-startupServingProbe; err != nil {
		t.Fatalf("web serving was not available before startup integrity recovery blocked: %v", err)
	}
	waitForWebCondition(t, func() bool { return calls.Load() >= 2 }, "watchdog ticks while startup integrity recovery is blocked")

	releaseStartup()
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
	var notifyCalls atomic.Int64
	systemdWatchdogNotify = func(string) error {
		notifyCalls.Add(1)
		return nil
	}
	sendRunWatchdogTick(t.Context(), &lastBeat, func(context.Context) error {
		return errors.New("web serving unavailable")
	})
	if got := notifyCalls.Load(); got != 0 {
		t.Fatalf("failed web-serving probe still sent %d watchdog notifications", got)
	}
	if got := lastBeat.Load(); got != oldBeat {
		t.Fatalf("failed web-serving probe updated heartbeat to %d, want %d", got, oldBeat)
	}

	systemdWatchdogNotify = func(string) error {
		return errors.New("notify failed")
	}
	sendRunWatchdogTick(t.Context(), &lastBeat)
	if got := lastBeat.Load(); got != oldBeat {
		t.Fatalf("failed watchdog notify updated heartbeat to %d, want %d", got, oldBeat)
	}

	systemdWatchdogNotify = func(string) error { return nil }
	sendRunWatchdogTick(t.Context(), &lastBeat)
	if got := lastBeat.Load(); got <= oldBeat {
		t.Fatalf("successful watchdog notify left heartbeat at %d, want newer than %d", got, oldBeat)
	}
}

func TestWatchdogSelfHealthStopsOnContextCancel(t *testing.T) {
	var lastBeat atomic.Int64
	lastBeat.Store(time.Now().UnixNano())
	ctx, cancel := context.WithCancel(t.Context())
	done := startWatchdogSelfHealth(ctx, 20*time.Millisecond, 10*time.Millisecond, &lastBeat)
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
		"abc.def.secret",
		"secret-cookie",
		"sensitive",
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
