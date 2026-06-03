//go:build linux

package kernel

import (
	"errors"
	"strings"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestEntryMode(t *testing.T) {
	tests := []struct {
		typeName string
		hash     string
		want     string
	}{
		{typeName: "hash:ip", hash: "net", want: "ip"},
		{typeName: "hash:net", hash: "ip", want: "net"},
		{typeName: "HASH:NET", hash: "ip", want: "net"},
		{typeName: "", hash: "net", want: "net"},
		{typeName: "", hash: "ip", want: "ip"},
	}
	for _, tt := range tests {
		if got := entryMode(tt.typeName, tt.hash); got != tt.want {
			t.Fatalf("entryMode(%q, %q) = %q, want %q", tt.typeName, tt.hash, got, tt.want)
		}
	}
}

func TestParseEntryHashIP(t *testing.T) {
	entry, err := parseEntry("ip", "1.2.3.4")
	if err != nil {
		t.Fatalf("parseEntry returned error: %v", err)
	}
	if entry.CIDR != 0 {
		t.Fatalf("expected CIDR 0 for hash:ip entry, got %d", entry.CIDR)
	}
	if got := entry.IP.String(); got != "1.2.3.4" {
		t.Fatalf("unexpected IP %q", got)
	}
}

func TestParseEntryHashNet(t *testing.T) {
	entry, err := parseEntry("net", "1.2.3.0/24")
	if err != nil {
		t.Fatalf("parseEntry returned error: %v", err)
	}
	if entry.CIDR != 24 {
		t.Fatalf("expected CIDR 24, got %d", entry.CIDR)
	}
	ipEntry, err := parseEntry("net", "1.2.3.4")
	if err != nil {
		t.Fatalf("parseEntry returned error: %v", err)
	}
	if ipEntry.CIDR != 32 {
		t.Fatalf("expected CIDR 32, got %d", ipEntry.CIDR)
	}
}

func TestParseEntryRejectsInvalidIPv4(t *testing.T) {
	tests := []struct {
		mode string
		line string
		want string
	}{
		{mode: "ip", line: "not-an-ip", want: `invalid IPv4 address "not-an-ip"`},
		{mode: "net", line: "2001:db8::/32", want: `invalid IPv4 CIDR "2001:db8::/32"`},
		{mode: "net", line: "not-an-ip", want: `invalid IPv4 address "not-an-ip"`},
	}
	for _, tt := range tests {
		t.Run(tt.mode+" "+tt.line, func(t *testing.T) {
			_, err := parseEntry(tt.mode, tt.line)
			if err == nil {
				t.Fatal("parseEntry() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseEntry() error = %q, want substring %q", err, tt.want)
			}
		})
	}
}

func TestTemporaryNameLength(t *testing.T) {
	got := temporaryName("firehol_level1")
	if len(got) > 31 {
		t.Fatalf("temporary name %q exceeds ipset limit", got)
	}
}

func TestMaxInt(t *testing.T) {
	if got := maxInt(1024, 20); got != 1024 {
		t.Fatalf("maxInt(1024, 20) = %d, want 1024", got)
	}
	if got := maxInt(20, 1024); got != 1024 {
		t.Fatalf("maxInt(20, 1024) = %d, want 1024", got)
	}
}

func TestLoadedSetsWithOps(t *testing.T) {
	ops := &fakeIPSetOps{
		sets: []netlink.IPSetResult{
			{SetName: "level1", TypeName: "hash:net", NumEntries: 2},
			{SetName: "allowlist", TypeName: "hash:ip", NumEntries: 3},
		},
	}

	got, err := loadedSets(ops)
	if err != nil {
		t.Fatalf("loadedSets() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("loadedSets() len = %d, want 2", len(got))
	}
	if got["level1"] != (SetInfo{Name: "level1", TypeName: "hash:net", Entries: 2}) {
		t.Fatalf("level1 info = %#v", got["level1"])
	}
	if got["allowlist"] != (SetInfo{Name: "allowlist", TypeName: "hash:ip", Entries: 3}) {
		t.Fatalf("allowlist info = %#v", got["allowlist"])
	}

	errList := errors.New("list failed")
	ops = &fakeIPSetOps{listErr: errList}
	if _, err := loadedSets(ops); !errors.Is(err, errList) {
		t.Fatalf("loadedSets() error = %v, want %v", err, errList)
	}
}

func TestApplyIfLoadedSkipsMissingSet(t *testing.T) {
	ops := &fakeIPSetOps{sets: []netlink.IPSetResult{{SetName: "other", TypeName: "hash:ip"}}}

	applied, err := applyIfLoaded(ops, "level1", "ip", []string{"192.0.2.1"})
	if err != nil {
		t.Fatalf("applyIfLoaded() error = %v", err)
	}
	if applied {
		t.Fatal("applyIfLoaded() applied missing set")
	}
	if len(ops.creates) != 0 || len(ops.adds) != 0 || len(ops.swaps) != 0 || len(ops.destroys) != 0 {
		t.Fatalf("missing set touched netlink operations: %#v", ops)
	}
}

func TestApplyIfLoadedReplacesLoadedSet(t *testing.T) {
	ops := &fakeIPSetOps{sets: []netlink.IPSetResult{{SetName: "level1", TypeName: "hash:net", NumEntries: 1}}}

	applied, err := applyIfLoaded(ops, "level1", "ip", []string{" 192.0.2.0/24 ", "", "198.51.100.1"})
	if err != nil {
		t.Fatalf("applyIfLoaded() error = %v", err)
	}
	if !applied {
		t.Fatal("applyIfLoaded() applied = false, want true")
	}
	if len(ops.creates) != 1 {
		t.Fatalf("create calls = %d, want 1", len(ops.creates))
	}
	create := ops.creates[0]
	if !strings.HasPrefix(create.name, "uip_") || len(create.name) > 31 {
		t.Fatalf("temporary set name = %q", create.name)
	}
	if create.typeName != "hash:net" {
		t.Fatalf("created type = %q, want hash:net", create.typeName)
	}
	if !create.options.Replace || create.options.Family != unix.AF_INET || create.options.MaxElements != 1024 {
		t.Fatalf("create options = %#v", create.options)
	}
	if len(ops.adds) != 2 {
		t.Fatalf("add calls = %d, want 2", len(ops.adds))
	}
	if ops.adds[0].ip != "192.0.2.0" || ops.adds[0].cidr != 24 {
		t.Fatalf("first add = %#v", ops.adds[0])
	}
	if ops.adds[1].ip != "198.51.100.1" || ops.adds[1].cidr != 32 {
		t.Fatalf("second add = %#v", ops.adds[1])
	}
	if len(ops.swaps) != 1 || ops.swaps[0] != [2]string{create.name, "level1"} {
		t.Fatalf("swap calls = %#v", ops.swaps)
	}
	if len(ops.destroys) != 1 || ops.destroys[0] != create.name {
		t.Fatalf("destroy calls = %#v, want [%q]", ops.destroys, create.name)
	}
}

func TestApplyIfLoadedUsesFallbackTypeForUnknownLoadedType(t *testing.T) {
	ops := &fakeIPSetOps{sets: []netlink.IPSetResult{{SetName: "level1"}}}

	applied, err := applyIfLoaded(ops, "level1", "net", []string{"203.0.113.1"})
	if err != nil {
		t.Fatalf("applyIfLoaded() error = %v", err)
	}
	if !applied {
		t.Fatal("applyIfLoaded() applied = false, want true")
	}
	if got := ops.creates[0].typeName; got != "hash:net" {
		t.Fatalf("created type = %q, want hash:net", got)
	}
}

func TestApplyIfLoadedErrorPaths(t *testing.T) {
	errCreate := errors.New("create failed")
	errAdd := errors.New("add failed")
	errSwap := errors.New("swap failed")

	tests := []struct {
		name          string
		lines         []string
		setup         func(*fakeIPSetOps)
		wantErr       error
		wantErrSubstr string
		wantDestroy   bool
	}{
		{
			name:    "create error does not destroy",
			lines:   []string{"192.0.2.1"},
			setup:   func(ops *fakeIPSetOps) { ops.createErr = errCreate },
			wantErr: errCreate,
		},
		{
			name:        "add error destroys temporary set",
			lines:       []string{"192.0.2.1"},
			setup:       func(ops *fakeIPSetOps) { ops.addErr = errAdd },
			wantErr:     errAdd,
			wantDestroy: true,
		},
		{
			name:        "swap error destroys temporary set",
			lines:       []string{"192.0.2.1"},
			setup:       func(ops *fakeIPSetOps) { ops.swapErr = errSwap },
			wantErr:     errSwap,
			wantDestroy: true,
		},
		{
			name:          "invalid entry destroys temporary set",
			lines:         []string{"2001:db8::1"},
			wantErrSubstr: `invalid IPv4 address "2001:db8::1"`,
			wantDestroy:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops := &fakeIPSetOps{sets: []netlink.IPSetResult{{SetName: "level1", TypeName: "hash:ip"}}}
			if tt.setup != nil {
				tt.setup(ops)
			}

			applied, err := applyIfLoaded(ops, "level1", "ip", tt.lines)
			if applied {
				t.Fatal("applyIfLoaded() applied = true, want false")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("applyIfLoaded() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErrSubstr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr)) {
				t.Fatalf("applyIfLoaded() error = %v, want substring %q", err, tt.wantErrSubstr)
			}
			if got := len(ops.destroys) > 0; got != tt.wantDestroy {
				t.Fatalf("destroy called = %v, want %v; calls=%#v", got, tt.wantDestroy, ops.destroys)
			}
		})
	}
}

type fakeIPSetOps struct {
	sets      []netlink.IPSetResult
	listErr   error
	createErr error
	addErr    error
	swapErr   error

	creates  []fakeCreateCall
	adds     []fakeAddCall
	swaps    [][2]string
	destroys []string
}

type fakeCreateCall struct {
	name     string
	typeName string
	options  netlink.IpsetCreateOptions
}

type fakeAddCall struct {
	name string
	ip   string
	cidr uint8
}

func (f *fakeIPSetOps) listAll() ([]netlink.IPSetResult, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.sets, nil
}

func (f *fakeIPSetOps) create(name, typeName string, options netlink.IpsetCreateOptions) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.creates = append(f.creates, fakeCreateCall{name: name, typeName: typeName, options: options})
	return nil
}

func (f *fakeIPSetOps) destroy(name string) error {
	f.destroys = append(f.destroys, name)
	return nil
}

func (f *fakeIPSetOps) add(name string, entry *netlink.IPSetEntry) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.adds = append(f.adds, fakeAddCall{name: name, ip: entry.IP.String(), cidr: entry.CIDR})
	return nil
}

func (f *fakeIPSetOps) swap(name, otherName string) error {
	if f.swapErr != nil {
		return f.swapErr
	}
	f.swaps = append(f.swaps, [2]string{name, otherName})
	return nil
}
