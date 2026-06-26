package engine

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/firehol/update-ipsets/pkg/asnloc"
)

func TestASNDatabaseCacheRetiresLeasedEntriesAfterRelease(t *testing.T) {
	cache := newASNDatabaseCache()
	lease, err := cache.acquire("asn", "/tmp/asn.source", 1, func() (*asnloc.Database, error) {
		return &asnloc.Database{Provider: "test"}, nil
	})
	if err != nil {
		t.Fatalf("acquire() error = %v", err)
	}
	entry := lease.entry

	retired := cache.retireAll()
	if len(retired) != 0 {
		t.Fatalf("retireAll() returned %d idle databases while entry was leased", len(retired))
	}
	if !entry.retired {
		t.Fatalf("entry was not marked retired")
	}
	if entry.closed {
		t.Fatalf("leased entry was closed before release")
	}
	if got, want := entry.refs, 1; got != want {
		t.Fatalf("entry refs = %d, want %d", got, want)
	}
	if got := len(cache.dbs); got != 0 {
		t.Fatalf("cache retained %d entries after retireAll(), want 0", got)
	}

	lease.Close()
	if !entry.closed {
		t.Fatalf("retired entry was not closed after final lease release")
	}
	if got, want := entry.refs, 0; got != want {
		t.Fatalf("entry refs after release = %d, want %d", got, want)
	}

	lease.Close()
	if got, want := entry.refs, 0; got != want {
		t.Fatalf("entry refs after duplicate release = %d, want %d", got, want)
	}
}

func TestASNDatabaseCacheReplacementKeepsOldLeaseOpenUntilRelease(t *testing.T) {
	cache := newASNDatabaseCache()
	oldLease, err := cache.acquire("asn", "/tmp/asn.source", 1, func() (*asnloc.Database, error) {
		return &asnloc.Database{Provider: "old"}, nil
	})
	if err != nil {
		t.Fatalf("initial acquire() error = %v", err)
	}
	oldEntry := oldLease.entry

	newLease, err := cache.acquire("asn", "/tmp/asn.source", 2, func() (*asnloc.Database, error) {
		return &asnloc.Database{Provider: "new"}, nil
	})
	if err != nil {
		t.Fatalf("replacement acquire() error = %v", err)
	}
	defer newLease.Close()

	if !oldEntry.retired {
		t.Fatalf("old entry was not retired after provider key changed")
	}
	if oldEntry.closed {
		t.Fatalf("old leased entry was closed before release")
	}
	if got, want := oldEntry.refs, 1; got != want {
		t.Fatalf("old entry refs = %d, want %d", got, want)
	}
	if newLease.entry == oldEntry {
		t.Fatalf("replacement reused retired entry")
	}
	if newLease.Database() == nil || newLease.Database().Provider != "new" {
		t.Fatalf("replacement lease database = %#v, want provider new", newLease.Database())
	}

	oldLease.Close()
	if !oldEntry.closed {
		t.Fatalf("old entry was not closed after release")
	}
}

func TestASNDatabaseCacheKeepsExistingEntryWhenReplacementOpenFails(t *testing.T) {
	cache := newASNDatabaseCache()
	lease, err := cache.acquire("asn", "/tmp/asn.source", 1, func() (*asnloc.Database, error) {
		return &asnloc.Database{Provider: "old"}, nil
	})
	if err != nil {
		t.Fatalf("initial acquire() error = %v", err)
	}
	defer lease.Close()
	entry := lease.entry

	wantErr := errors.New("open failed")
	if _, err := cache.acquire("asn", "/tmp/asn.source", 2, func() (*asnloc.Database, error) {
		return nil, wantErr
	}); !errors.Is(err, wantErr) {
		t.Fatalf("replacement acquire() error = %v, want %v", err, wantErr)
	}
	if entry.retired {
		t.Fatalf("existing entry was retired after failed replacement open")
	}
	if entry.closed {
		t.Fatalf("existing entry was closed after failed replacement open")
	}
	if got := cache.dbs["asn"]; got != entry {
		t.Fatalf("cache entry after failed replacement = %#v, want original entry", got)
	}
}

func TestASNDatabaseCacheOpenDoesNotBlockIndependentProvider(t *testing.T) {
	cache := newASNDatabaseCache()
	firstOpenStarted := make(chan struct{})
	releaseFirstOpen := make(chan struct{})
	firstDone := make(chan error, 1)

	go func() {
		lease, err := cache.acquire("asn-a", "/tmp/asn-a.source", 1, func() (*asnloc.Database, error) {
			close(firstOpenStarted)
			<-releaseFirstOpen
			return &asnloc.Database{Provider: "asn-a"}, nil
		})
		if lease != nil {
			lease.Close()
		}
		firstDone <- err
	}()

	select {
	case <-firstOpenStarted:
	case <-time.After(time.Second):
		t.Fatal("first provider open did not start")
	}

	secondDone := make(chan error, 1)
	go func() {
		lease, err := cache.acquire("asn-b", "/tmp/asn-b.source", 1, func() (*asnloc.Database, error) {
			return &asnloc.Database{Provider: "asn-b"}, nil
		})
		if lease != nil {
			lease.Close()
		}
		secondDone <- err
	}()

	select {
	case err := <-secondDone:
		if err != nil {
			close(releaseFirstOpen)
			t.Fatalf("second provider acquire error = %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		close(releaseFirstOpen)
		t.Fatal("independent provider acquire was blocked by another provider open")
	}

	close(releaseFirstOpen)
	if err := <-firstDone; err != nil {
		t.Fatalf("first provider acquire error = %v", err)
	}
}

func TestASNDatabaseCacheDeduplicatesConcurrentSameProviderOpen(t *testing.T) {
	cache := newASNDatabaseCache()
	firstOpenStarted := make(chan struct{})
	releaseFirstOpen := make(chan struct{})
	secondOpenCalled := make(chan struct{})
	firstDone := make(chan error, 1)
	var openCalls atomic.Int64

	go func() {
		lease, err := cache.acquire("asn", "/tmp/asn.source", 1, func() (*asnloc.Database, error) {
			openCalls.Add(1)
			close(firstOpenStarted)
			<-releaseFirstOpen
			return &asnloc.Database{Provider: "asn"}, nil
		})
		if lease != nil {
			lease.Close()
		}
		firstDone <- err
	}()

	select {
	case <-firstOpenStarted:
	case <-time.After(time.Second):
		t.Fatal("first provider open did not start")
	}

	secondDone := make(chan error, 1)
	go func() {
		lease, err := cache.acquire("asn", "/tmp/asn.source", 1, func() (*asnloc.Database, error) {
			openCalls.Add(1)
			close(secondOpenCalled)
			return &asnloc.Database{Provider: "asn-duplicate"}, nil
		})
		if lease != nil {
			lease.Close()
		}
		secondDone <- err
	}()

	select {
	case <-secondOpenCalled:
		close(releaseFirstOpen)
		t.Fatal("duplicate provider acquire opened the same cache key concurrently")
	case <-time.After(200 * time.Millisecond):
	}

	close(releaseFirstOpen)
	if err := <-firstDone; err != nil {
		t.Fatalf("first provider acquire error = %v", err)
	}
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatalf("second provider acquire error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second provider acquire did not finish after first open completed")
	}
	if got, want := openCalls.Load(), int64(1); got != want {
		t.Fatalf("open calls = %d, want %d", got, want)
	}
}

func TestASNDatabaseCacheAcquireContextCancelsWhileWaitingForSameProviderOpen(t *testing.T) {
	cache := newASNDatabaseCache()
	key := asnDatabaseCacheKey{provider: "asn", path: "/tmp/asn.source", sizeModKey: 1}
	load := &asnDatabaseLoad{done: make(chan struct{})}
	cache.mu.Lock()
	cache.ensureLocked()
	cache.loads[key] = load
	cache.mu.Unlock()

	secondOpenCalled := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	secondDone := make(chan error, 1)
	go func() {
		lease, err := cache.acquireContext(ctx, "asn", "/tmp/asn.source", 1, func() (*asnloc.Database, error) {
			close(secondOpenCalled)
			return &asnloc.Database{Provider: "asn-duplicate"}, nil
		})
		if lease != nil {
			lease.Close()
		}
		secondDone <- err
	}()

	cancel()
	select {
	case <-secondOpenCalled:
		t.Fatal("canceled same-provider waiter opened a duplicate database")
	case err := <-secondDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled same-provider waiter error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled same-provider waiter did not return")
	}
}

func TestASNDatabaseCacheSurvivesConcurrentAcquireAndRetire(t *testing.T) {
	cache := newASNDatabaseCache()
	const (
		workers    = 8
		iterations = 100
	)

	var opened atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, workers)

	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			<-start
			for i := 0; i < iterations; i++ {
				key := int64((worker + i) % 3)
				lease, err := cache.acquire("asn", "/tmp/asn.source", key, func() (*asnloc.Database, error) {
					id := opened.Add(1)
					return &asnloc.Database{Provider: fmt.Sprintf("db-%d", id)}, nil
				})
				if err != nil {
					errs <- fmt.Errorf("worker %d acquire %d: %w", worker, i, err)
					return
				}
				if lease.Database() == nil {
					errs <- fmt.Errorf("worker %d acquire %d returned nil database", worker, i)
					lease.Close()
					return
				}
				if i%2 == 0 {
					runtime.Gosched()
				}
				lease.Close()
			}
		}(worker)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			closeASNLookupDatabases(cache.retireAll(), nil)
			runtime.Gosched()
		}
	}()

	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	closeASNLookupDatabases(cache.retireAll(), nil)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if got := len(cache.dbs); got != 0 {
		t.Fatalf("cache retained %d entries after final retireAll(), want 0", got)
	}
}
