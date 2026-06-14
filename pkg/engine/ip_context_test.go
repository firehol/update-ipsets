package engine

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

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
