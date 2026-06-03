//go:build linux

package kernel

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

type SetInfo struct {
	Name     string
	TypeName string
	Entries  uint32
}

type ipsetOps interface {
	listAll() ([]netlink.IPSetResult, error)
	create(name, typeName string, options netlink.IpsetCreateOptions) error
	destroy(name string) error
	add(name string, entry *netlink.IPSetEntry) error
	swap(name, otherName string) error
}

type netlinkIPSetOps struct{}

func (netlinkIPSetOps) listAll() ([]netlink.IPSetResult, error) {
	return netlink.IpsetListAll()
}

func (netlinkIPSetOps) create(name, typeName string, options netlink.IpsetCreateOptions) error {
	return netlink.IpsetCreate(name, typeName, options)
}

func (netlinkIPSetOps) destroy(name string) error {
	return netlink.IpsetDestroy(name)
}

func (netlinkIPSetOps) add(name string, entry *netlink.IPSetEntry) error {
	return netlink.IpsetAdd(name, entry)
}

func (netlinkIPSetOps) swap(name, otherName string) error {
	return netlink.IpsetSwap(name, otherName)
}

func LoadedSets() (map[string]SetInfo, error) {
	return loadedSets(netlinkIPSetOps{})
}

func loadedSets(ops ipsetOps) (map[string]SetInfo, error) {
	results, err := ops.listAll()
	if err != nil {
		return nil, err
	}
	out := make(map[string]SetInfo, len(results))
	for _, result := range results {
		out[result.SetName] = SetInfo{
			Name:     result.SetName,
			TypeName: result.TypeName,
			Entries:  result.NumEntries,
		}
	}
	return out, nil
}

func ApplyIfLoaded(name, hash string, lines []string) (bool, error) {
	return applyIfLoaded(netlinkIPSetOps{}, name, hash, lines)
}

func applyIfLoaded(ops ipsetOps, name, hash string, lines []string) (bool, error) {
	loaded, err := loadedSets(ops)
	if err != nil {
		return false, err
	}
	target, ok := loaded[name]
	if !ok {
		return false, nil
	}

	mode := entryMode(target.TypeName, hash)
	typeName := target.TypeName
	if typeName == "" {
		typeName = "hash:" + mode
	}
	tmpName := temporaryName(name)
	options := netlink.IpsetCreateOptions{
		Replace:     true,
		Family:      unix.AF_INET,
		MaxElements: uint32(maxInt(1024, len(lines)*2+16)),
	}
	if err := ops.create(tmpName, typeName, options); err != nil {
		return false, err
	}
	defer func() { _ = ops.destroy(tmpName) }()

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entry, err := parseEntry(mode, line)
		if err != nil {
			return false, err
		}
		if err := ops.add(tmpName, entry); err != nil {
			return false, err
		}
	}
	if err := ops.swap(tmpName, name); err != nil {
		return false, err
	}
	return true, nil
}

func entryMode(typeName, fallback string) string {
	lower := strings.ToLower(typeName)
	switch {
	case strings.Contains(lower, "hash:net"):
		return "net"
	case strings.Contains(lower, "hash:ip"):
		return "ip"
	case strings.EqualFold(fallback, "net"):
		return "net"
	default:
		return "ip"
	}
}

func parseEntry(mode, line string) (*netlink.IPSetEntry, error) {
	switch mode {
	case "net":
		if strings.Contains(line, "/") {
			ip, network, err := net.ParseCIDR(line)
			if err != nil || ip.To4() == nil {
				return nil, fmt.Errorf("invalid IPv4 CIDR %q", line)
			}
			ones, _ := network.Mask.Size()
			return &netlink.IPSetEntry{IP: network.IP.To4(), CIDR: uint8(ones)}, nil
		}
		ip := net.ParseIP(line).To4()
		if ip == nil {
			return nil, fmt.Errorf("invalid IPv4 address %q", line)
		}
		return &netlink.IPSetEntry{IP: ip, CIDR: 32}, nil
	default:
		ip := net.ParseIP(line).To4()
		if ip == nil {
			return nil, fmt.Errorf("invalid IPv4 address %q", line)
		}
		return &netlink.IPSetEntry{IP: ip}, nil
	}
}

func temporaryName(name string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", name, time.Now().UnixNano())))
	return "uip_" + hex.EncodeToString(sum[:])[:24]
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
