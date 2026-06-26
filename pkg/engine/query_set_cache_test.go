package engine

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/config"
	"github.com/firehol/update-ipsets/pkg/iprange"
)

func TestOpenLatestSetForQueryHonorsCancelledContext(t *testing.T) {
	eng := newEngineFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	src, release, err := eng.openLatestSetForQuery(ctx, "alpha")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("openLatestSetForQuery() error = %v, want context.Canceled", err)
	}
	if src != nil {
		t.Fatalf("openLatestSetForQuery() src = %#v, want nil", src)
	}
	if release != nil {
		t.Fatal("openLatestSetForQuery() release should be nil on cancellation")
	}
}

func TestSharedLatestSetCacheSlowOpenDoesNotBlockDifferentCachedFeed(t *testing.T) {
	cfg := config.New()
	cfg.Sources["slow"] = &config.Source{Name: "slow"}
	cfg.Sources["fast"] = &config.Source{Name: "fast"}
	eng := newEngineFixture(t, withConfig(cfg))
	writeLatestSetForQueryCacheTest(t, eng, "slow", "10.0.0.1\n")
	writeLatestSetForQueryCacheTest(t, eng, "fast", "10.0.0.2\n")

	src, release, err := eng.openLatestSetForQuery(t.Context(), "fast")
	if err != nil {
		t.Fatalf("prime fast cache: %v", err)
	}
	if src == nil {
		t.Fatal("prime fast cache returned nil source")
	}
	release()

	slowStarted := make(chan struct{})
	releaseSlow := make(chan struct{})
	var slowStartedOnce sync.Once
	restore := setOpenLatestSetHookForTest(func(name string) {
		if name != "slow" {
			return
		}
		slowStartedOnce.Do(func() { close(slowStarted) })
		<-releaseSlow
	})
	defer restore()

	slowDone := make(chan error, 1)
	go func() {
		src, release, err := eng.openLatestSetForQuery(t.Context(), "slow")
		if release != nil {
			defer release()
		}
		if err == nil && src == nil {
			err = errors.New("slow open returned nil source")
		}
		slowDone <- err
	}()

	select {
	case <-slowStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("slow latest-set open did not start")
	}

	fastDone := make(chan error, 1)
	go func() {
		src, release, err := eng.openLatestSetForQuery(t.Context(), "fast")
		if release != nil {
			defer release()
		}
		if err == nil && src == nil {
			err = errors.New("fast cached open returned nil source")
		}
		fastDone <- err
	}()

	select {
	case err := <-fastDone:
		if err != nil {
			t.Fatalf("cached fast lookup while slow feed opens: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		close(releaseSlow)
		t.Fatal("cached fast lookup blocked behind slow feed open")
	}

	close(releaseSlow)
	if err := <-slowDone; err != nil {
		t.Fatalf("slow latest-set open: %v", err)
	}
}

func TestSharedLatestSetCacheSameFeedWaitHonorsContext(t *testing.T) {
	cfg := config.New()
	cfg.Sources["slow"] = &config.Source{Name: "slow"}
	eng := newEngineFixture(t, withConfig(cfg))
	writeLatestSetForQueryCacheTest(t, eng, "slow", "10.0.0.1\n")

	slowStarted := make(chan struct{})
	releaseSlow := make(chan struct{})
	var slowStartedOnce sync.Once
	restore := setOpenLatestSetHookForTest(func(name string) {
		if name != "slow" {
			return
		}
		slowStartedOnce.Do(func() { close(slowStarted) })
		<-releaseSlow
	})
	defer restore()

	firstDone := make(chan error, 1)
	go func() {
		src, release, err := eng.openLatestSetForQuery(t.Context(), "slow")
		if release != nil {
			defer release()
		}
		if err == nil && src == nil {
			err = errors.New("first slow open returned nil source")
		}
		firstDone <- err
	}()

	select {
	case <-slowStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first slow latest-set open did not start")
	}

	ctx, cancel := context.WithCancel(t.Context())
	secondDone := make(chan error, 1)
	go func() {
		src, release, err := eng.openLatestSetForQuery(ctx, "slow")
		if release != nil {
			defer release()
		}
		if !errors.Is(err, context.Canceled) {
			secondDone <- err
			return
		}
		if src != nil {
			secondDone <- errors.New("second same-feed open returned source after cancellation")
			return
		}
		secondDone <- nil
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-secondDone:
		if err != nil {
			close(releaseSlow)
			t.Fatalf("second same-feed open after cancellation: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		close(releaseSlow)
		t.Fatal("second same-feed open did not unblock on context cancellation")
	}

	close(releaseSlow)
	if err := <-firstDone; err != nil {
		t.Fatalf("first slow latest-set open: %v", err)
	}
}

func TestSharedLatestSetCacheInvalidationDuringOpenRetries(t *testing.T) {
	cfg := config.New()
	cfg.Sources["slow"] = &config.Source{Name: "slow"}
	eng := newEngineFixture(t, withConfig(cfg))
	writeLatestSetForQueryCacheTest(t, eng, "slow", "10.0.0.1\n")

	firstOpenStarted := make(chan struct{})
	releaseFirstOpen := make(chan struct{})
	var openCalls atomic.Int32
	var firstOpenOnce sync.Once
	restore := setOpenLatestSetHookForTest(func(name string) {
		if name != "slow" {
			return
		}
		if openCalls.Add(1) == 1 {
			firstOpenOnce.Do(func() { close(firstOpenStarted) })
			<-releaseFirstOpen
		}
	})
	defer restore()

	openDone := make(chan *closableSource, 1)
	openErr := make(chan error, 1)
	go func() {
		src, release, err := eng.openLatestSetForQuery(t.Context(), "slow")
		if release != nil {
			defer release()
		}
		if err != nil {
			openErr <- err
			return
		}
		openDone <- src
	}()

	select {
	case <-firstOpenStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first slow latest-set open did not start")
	}

	eng.querySetCache.Invalidate("slow")
	writeLatestSetForQueryCacheTest(t, eng, "slow", "10.0.0.3\n")
	close(releaseFirstOpen)

	select {
	case err := <-openErr:
		t.Fatalf("open after invalidation: %v", err)
	case src := <-openDone:
		if src == nil {
			t.Fatal("open after invalidation returned nil source")
		}
		ip, err := iprange.ParseIPv4Token("10.0.0.3")
		if err != nil {
			t.Fatal(err)
		}
		if !src.Contains(ip) {
			t.Fatal("open after invalidation did not return the refreshed set")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("open after invalidation did not finish")
	}
	if got := openCalls.Load(); got < 2 {
		t.Fatalf("latest-set open calls = %d, want at least 2 after invalidation retry", got)
	}
}

func writeLatestSetForQueryCacheTest(t *testing.T, eng *Engine, name, body string) {
	t.Helper()
	set, err := iprange.ParseReader(t.Context(), name, strings.NewReader(body), iprange.DefaultParseOptions())
	if err != nil {
		t.Fatalf("parse latest set for %s: %v", name, err)
	}
	set.Optimize()
	path := filepath.Join(eng.runtime.LibDir, name, "latest")
	if err := writeBinaryPath(path, set, time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatalf("write latest set for %s: %v", name, err)
	}
}
