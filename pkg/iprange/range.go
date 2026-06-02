package iprange

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	BinaryHeaderV10 = "iprange binary format v1.0\n"
	DefaultName     = "stdin"
)

var (
	ErrInvalidRange         = errors.New("invalid range")
	ErrInvalidIPv4          = errors.New("invalid IPv4 address")
	ErrInvalidIPv6          = errors.New("invalid IPv6 address")
	ErrInvalidPrefix        = errors.New("invalid prefix")
	ErrSingleIPsRangeTooBig = errors.New("range too large for single IP output")
	nativeEndian            = detectNativeEndian()
)

// Range represents a single inclusive IPv4 range stored in host byte order.
type Range struct {
	Lo uint32
	Hi uint32
}

func (r Range) Valid() bool {
	return r.Lo <= r.Hi
}

func (r Range) Size() uint64 {
	if !r.Valid() {
		return 0
	}
	return uint64(r.Hi) - uint64(r.Lo) + 1
}

func (r Range) String() string {
	if r.Lo == r.Hi {
		return Uint32ToIPv4(r.Lo)
	}
	return fmt.Sprintf("%s-%s", Uint32ToIPv4(r.Lo), Uint32ToIPv4(r.Hi))
}

type AddressFamily string

const (
	FamilyIPv4 AddressFamily = "ipv4"
	FamilyIPv6 AddressFamily = "ipv6"
)

func detectNativeEndian() binary.ByteOrder {
	return binary.NativeEndian
}

func Uint32ToIPv4(v uint32) string {
	var buf [15]byte
	n := 0
	n += writeUint32Dec(buf[n:], v>>24)
	buf[n] = '.'
	n++
	n += writeUint32Dec(buf[n:], (v>>16)&0xff)
	buf[n] = '.'
	n++
	n += writeUint32Dec(buf[n:], (v>>8)&0xff)
	buf[n] = '.'
	n++
	n += writeUint32Dec(buf[n:], v&0xff)
	return string(buf[:n])
}

func writeUint32Dec(buf []byte, v uint32) int {
	if v < 10 {
		buf[0] = byte('0' + v)
		return 1
	}
	if v < 100 {
		buf[1] = byte('0' + v%10)
		buf[0] = byte('0' + v/10)
		return 2
	}

	var tmp [3]byte
	i := 0
	for v > 0 {
		tmp[i] = byte('0' + v%10)
		v /= 10
		i++
	}
	for j := 0; j < i; j++ {
		buf[j] = tmp[i-1-j]
	}
	return i
}

func Netmask(prefix int) uint32 {
	if prefix <= 0 {
		return 0
	}
	if prefix >= 32 {
		return math.MaxUint32
	}
	return ^uint32(0) << (32 - prefix)
}

func Network(addr uint32, prefix int) uint32 {
	return addr & Netmask(prefix)
}

func Broadcast(addr uint32, prefix int) uint32 {
	return addr | ^Netmask(prefix)
}

func ParseIPv4Token(token string) (uint32, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) == 0 || len(parts) > 4 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidIPv4, token)
	}

	values := make([]uint64, len(parts))
	for i, part := range parts {
		if part == "" {
			return 0, fmt.Errorf("%w: %q", ErrInvalidIPv4, token)
		}
		v, err := strconv.ParseUint(part, 0, 32)
		if err != nil {
			return 0, fmt.Errorf("%w: %q", ErrInvalidIPv4, token)
		}
		values[i] = v
	}

	switch len(values) {
	case 1:
		return uint32(values[0]), nil
	case 2:
		if values[0] > 0xff || values[1] > 0xffffff {
			return 0, fmt.Errorf("%w: %q", ErrInvalidIPv4, token)
		}
		return uint32(values[0]<<24 | values[1]), nil
	case 3:
		if values[0] > 0xff || values[1] > 0xff || values[2] > 0xffff {
			return 0, fmt.Errorf("%w: %q", ErrInvalidIPv4, token)
		}
		return uint32(values[0]<<24 | values[1]<<16 | values[2]), nil
	case 4:
		for _, v := range values {
			if v > 0xff {
				return 0, fmt.Errorf("%w: %q", ErrInvalidIPv4, token)
			}
		}
		return uint32(values[0]<<24 | values[1]<<16 | values[2]<<8 | values[3]), nil
	default:
		return 0, fmt.Errorf("%w: %q", ErrInvalidIPv4, token)
	}
}

func ParsePrefix(token string) (int, error) {
	if token == "" {
		return 0, ErrInvalidPrefix
	}
	if n, err := strconv.Atoi(token); err == nil {
		if n < 0 || n > 32 {
			return 0, ErrInvalidPrefix
		}
		return n, nil
	}

	mask, err := ParseIPv4Token(token)
	if err != nil {
		return 0, ErrInvalidPrefix
	}

	var prefix int
	seenZero := false
	for bit := 31; bit >= 0; bit-- {
		isOne := mask&(1<<bit) != 0
		if seenZero && isOne {
			return 0, ErrInvalidPrefix
		}
		if isOne {
			prefix++
			continue
		}
		seenZero = true
	}
	return prefix, nil
}
