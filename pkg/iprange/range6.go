package iprange

import (
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
)

// Uint128 is the public 128-bit integer used by the IPv6 set API.
// It is represented as two uint64 halves in host order.
type Uint128 = uint128

// uint128 represents an unsigned 128-bit integer as two uint64 halves.
// Hi holds the most significant 64 bits, Lo the least significant.
type uint128 struct {
	Hi uint64
	Lo uint64
}

var (
	uint128Zero = uint128{}
	uint128Max  = uint128{Hi: math.MaxUint64, Lo: math.MaxUint64}
	uint128One  = uint128{Lo: 1}
)

func Uint128FromUint64(v uint64) Uint128 {
	return u128FromUint64(v)
}

func Uint128FromHiLo(hi, lo uint64) Uint128 {
	return u128FromHiLo(hi, lo)
}

func ParseUint128(token string) (Uint128, error) {
	return parseUint128(token)
}

func u128FromUint64(v uint64) uint128 {
	return uint128{Lo: v}
}

func u128FromHiLo(hi, lo uint64) uint128 {
	return uint128{Hi: hi, Lo: lo}
}

func u128FromBytes(b []byte) uint128 {
	if len(b) < 16 {
		return uint128Zero
	}
	return uint128{
		Hi: binary.BigEndian.Uint64(b[0:8]),
		Lo: binary.BigEndian.Uint64(b[8:16]),
	}
}

func (a uint128) IsZero() bool { return a.Hi == 0 && a.Lo == 0 }

func (a uint128) IsMax() bool {
	return a.Hi == math.MaxUint64 && a.Lo == math.MaxUint64
}

func (a uint128) Equals(b uint128) bool {
	return a.Hi == b.Hi && a.Lo == b.Lo
}

func (a uint128) LessThan(b uint128) bool {
	if a.Hi != b.Hi {
		return a.Hi < b.Hi
	}
	return a.Lo < b.Lo
}

func (a uint128) LessOrEqual(b uint128) bool {
	if a.Hi != b.Hi {
		return a.Hi < b.Hi
	}
	return a.Lo <= b.Lo
}

func (a uint128) GreaterThan(b uint128) bool {
	return b.LessThan(a)
}

func (a uint128) Cmp(b uint128) int {
	if a.Hi != b.Hi {
		if a.Hi < b.Hi {
			return -1
		}
		return 1
	}
	if a.Lo < b.Lo {
		return -1
	}
	if a.Lo > b.Lo {
		return 1
	}
	return 0
}

func (a uint128) Add(b uint128) uint128 {
	lo := a.Lo + b.Lo
	hi := a.Hi + b.Hi
	if lo < a.Lo {
		hi++
	}
	return uint128{Hi: hi, Lo: lo}
}

func (a uint128) Add64(b uint64) uint128 {
	lo := a.Lo + b
	hi := a.Hi
	if lo < a.Lo {
		hi++
	}
	return uint128{Hi: hi, Lo: lo}
}

func (a uint128) Sub(b uint128) uint128 {
	lo := a.Lo - b.Lo
	hi := a.Hi - b.Hi
	if a.Lo < b.Lo {
		hi--
	}
	return uint128{Hi: hi, Lo: lo}
}

func (a uint128) Sub64(b uint64) uint128 {
	lo := a.Lo - b
	hi := a.Hi
	if a.Lo < b {
		hi--
	}
	return uint128{Hi: hi, Lo: lo}
}

func (a uint128) Incr() uint128 { return a.Add64(1) }

func (a uint128) Decr() uint128 { return a.Sub64(1) }

func (a uint128) And(b uint128) uint128 {
	return uint128{Hi: a.Hi & b.Hi, Lo: a.Lo & b.Lo}
}

func (a uint128) Or(b uint128) uint128 {
	return uint128{Hi: a.Hi | b.Hi, Lo: a.Lo | b.Lo}
}

func (a uint128) Xor(b uint128) uint128 {
	return uint128{Hi: a.Hi ^ b.Hi, Lo: a.Lo ^ b.Lo}
}

func (a uint128) Not() uint128 {
	return uint128{Hi: ^a.Hi, Lo: ^a.Lo}
}

func (a uint128) Lsh(n uint) uint128 {
	if n == 0 {
		return a
	}
	if n >= 128 {
		return uint128Zero
	}
	if n >= 64 {
		return uint128{Hi: a.Lo << (n - 64), Lo: 0}
	}
	return uint128{
		Hi: (a.Hi << n) | (a.Lo >> (64 - n)),
		Lo: a.Lo << n,
	}
}

func (a uint128) Rsh(n uint) uint128 {
	if n == 0 {
		return a
	}
	if n >= 128 {
		return uint128Zero
	}
	if n >= 64 {
		return uint128{Hi: 0, Lo: a.Hi >> (n - 64)}
	}
	return uint128{
		Hi: a.Hi >> n,
		Lo: (a.Lo >> n) | (a.Hi << (64 - n)),
	}
}

func (a uint128) SetBit(bit int, val bool) uint128 {
	if bit < 1 || bit > 128 {
		return a
	}
	mask := uint128One.Lsh(uint(128 - bit))
	if val {
		return a.Or(mask)
	}
	return a.And(mask.Not())
}

func (a uint128) PutBytes(b []byte) {
	binary.BigEndian.PutUint64(b[0:8], a.Hi)
	binary.BigEndian.PutUint64(b[8:16], a.Lo)
}

func (a uint128) String() string {
	if a.IsZero() {
		return "0"
	}
	buf := make([]byte, 0, 40)
	v := a
	for !v.IsZero() {
		q, r := divMod10(v)
		buf = append(buf, '0'+byte(r))
		v = q
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

func divMod10(v uint128) (uint128, uint64) {
	var q uint128
	var r uint64

	r = v.Hi >> 56
	q.Hi = (r / 10) << 56
	r = r % 10

	r = (r << 8) | ((v.Hi >> 48) & 0xFF)
	q.Hi |= (r / 10) << 48
	r = r % 10

	r = (r << 8) | ((v.Hi >> 40) & 0xFF)
	q.Hi |= (r / 10) << 40
	r = r % 10

	r = (r << 8) | ((v.Hi >> 32) & 0xFF)
	q.Hi |= (r / 10) << 32
	r = r % 10

	r = (r << 8) | ((v.Hi >> 24) & 0xFF)
	q.Hi |= (r / 10) << 24
	r = r % 10

	r = (r << 8) | ((v.Hi >> 16) & 0xFF)
	q.Hi |= (r / 10) << 16
	r = r % 10

	r = (r << 8) | ((v.Hi >> 8) & 0xFF)
	q.Hi |= (r / 10) << 8
	r = r % 10

	r = (r << 8) | (v.Hi & 0xFF)
	q.Hi |= r / 10
	r = r % 10

	r = (r << 8) | ((v.Lo >> 56) & 0xFF)
	q.Lo = (r / 10) << 56
	r = r % 10

	r = (r << 8) | ((v.Lo >> 48) & 0xFF)
	q.Lo |= (r / 10) << 48
	r = r % 10

	r = (r << 8) | ((v.Lo >> 40) & 0xFF)
	q.Lo |= (r / 10) << 40
	r = r % 10

	r = (r << 8) | ((v.Lo >> 32) & 0xFF)
	q.Lo |= (r / 10) << 32
	r = r % 10

	r = (r << 8) | ((v.Lo >> 24) & 0xFF)
	q.Lo |= (r / 10) << 24
	r = r % 10

	r = (r << 8) | ((v.Lo >> 16) & 0xFF)
	q.Lo |= (r / 10) << 16
	r = r % 10

	r = (r << 8) | ((v.Lo >> 8) & 0xFF)
	q.Lo |= (r / 10) << 8
	r = r % 10

	r = (r << 8) | (v.Lo & 0xFF)
	q.Lo |= r / 10
	r = r % 10

	return q, r
}

func parseUint128(s string) (uint128, error) {
	if s == "" {
		return uint128Zero, fmt.Errorf("empty uint128 string")
	}
	var result uint128
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return uint128Zero, fmt.Errorf("non-digit %q in uint128", ch)
		}
		d := uint64(ch - '0')
		prev := result
		result = mulAdd10(result, d)
		if result.LessThan(prev) {
			return uint128Zero, fmt.Errorf("uint128 overflow parsing %q", s)
		}
	}
	return result, nil
}

func mulAdd10(v uint128, d uint64) uint128 {
	lo := v.Lo * 10
	hi := v.Hi*10 + mulHi64(v.Lo, 10)
	sum := lo + d
	if sum < lo {
		hi++
	}
	return uint128{Hi: hi, Lo: sum}
}

func mulHi64(a, b uint64) uint64 {
	aHi := a >> 32
	aLo := a & 0xFFFFFFFF
	bHi := b >> 32
	bLo := b & 0xFFFFFFFF
	cross := aHi*bLo + aLo*bHi
	crossHi := cross >> 32
	return aHi*bHi + crossHi
}

// Range6 represents a single inclusive IPv6 range.
type Range6 struct {
	Lo Uint128
	Hi Uint128
}

func (r Range6) Valid() bool {
	return r.Lo.LessOrEqual(r.Hi)
}

func (r Range6) Size() Uint128 {
	if !r.Valid() {
		return uint128Zero
	}
	return r.Hi.Sub(r.Lo).Incr()
}

func (r Range6) String() string {
	if r.Lo.Equals(r.Hi) {
		return Uint128ToIPv6(r.Lo)
	}
	return fmt.Sprintf("%s-%s", Uint128ToIPv6(r.Lo), Uint128ToIPv6(r.Hi))
}

func Netmask6(prefix int) Uint128 {
	if prefix <= 0 {
		return uint128Zero
	}
	if prefix >= 128 {
		return uint128Max
	}
	return uint128Max.Lsh(uint(128 - prefix))
}

func Broadcast6(addr Uint128, prefix int) Uint128 {
	return addr.Or(Netmask6(prefix).Not())
}

func Network6(addr Uint128, prefix int) Uint128 {
	return addr.And(Netmask6(prefix))
}

func Uint128ToIPv6(v Uint128) string {
	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:8], v.Hi)
	binary.BigEndian.PutUint64(buf[8:16], v.Lo)

	var groups [8]uint16
	for i := 0; i < 8; i++ {
		groups[i] = uint16(buf[i*2])<<8 | uint16(buf[i*2+1])
	}

	bestStart, bestLen := 0, 0
	curStart, curLen := -1, 0
	for i := 0; i < 8; i++ {
		if groups[i] == 0 {
			if curStart == -1 {
				curStart = i
			}
			curLen = i - curStart + 1
			if curLen > bestLen {
				bestStart = curStart
				bestLen = curLen
			}
		} else {
			curStart = -1
		}
	}

	var out [45]byte
	n := 0
	for i := 0; i < 8; {
		if i == bestStart && bestLen >= 2 {
			out[n] = ':'
			out[n+1] = ':'
			n += 2
			i += bestLen
			continue
		}
		if n > 0 && out[n-1] != ':' {
			out[n] = ':'
			n++
		}
		n += writeHexUint16(out[n:], groups[i])
		i++
	}

	return string(out[:n])
}

const hexDigits = "0123456789abcdef"

func writeHexUint16(buf []byte, v uint16) int {
	if v == 0 {
		buf[0] = '0'
		return 1
	}
	var tmp [4]byte
	i := 0
	for v > 0 {
		tmp[i] = hexDigits[v&0xf]
		v >>= 4
		i++
	}
	for j := 0; j < i; j++ {
		buf[j] = tmp[i-1-j]
	}
	return i
}

func ParseIPv6Token(token string) (Uint128, error) {
	ip := net.ParseIP(token)
	if ip == nil {
		return uint128Zero, fmt.Errorf("%w: %q", ErrInvalidIPv6, token)
	}
	ip = ip.To16()
	if ip == nil {
		return uint128Zero, fmt.Errorf("%w: %q", ErrInvalidIPv6, token)
	}
	return u128FromBytes(ip), nil
}

func ParsePrefix6(token string) (int, error) {
	if token == "" {
		return 0, ErrInvalidPrefix
	}
	n, err := strconv.Atoi(token)
	if err != nil {
		return 0, ErrInvalidPrefix
	}
	if n < 0 || n > 128 {
		return 0, ErrInvalidPrefix
	}
	return n, nil
}

func IPv4ToMapped6(ip uint32) Uint128 {
	return uint128{Hi: 0, Lo: uint64(0x0000FFFF)<<32 | uint64(ip)}
}

func IsIPv4Mapped6(addr Uint128) bool {
	return addr.Hi == 0 && (addr.Lo>>32) == 0x0000FFFF
}

func Mapped6ToIPv4(addr Uint128) (uint32, bool) {
	if !IsIPv4Mapped6(addr) {
		return 0, false
	}
	return uint32(addr.Lo), true
}

func parseIPv6Endpoint(token string, opts ParseOptions) (Uint128, Uint128, error) {
	if strings.Contains(token, "/") {
		parts := strings.SplitN(token, "/", 2)
		addr, err := ParseIPv6Token(parts[0])
		if err != nil {
			return uint128Zero, uint128Zero, err
		}
		prefix, err := ParsePrefix6(parts[1])
		if err != nil {
			return uint128Zero, uint128Zero, err
		}
		lo := addr
		if opts.UseCIDRNetwork {
			lo = Network6(addr, prefix)
		}
		return lo, Broadcast6(lo, prefix), nil
	}

	addr, err := ParseIPv6Token(token)
	if err != nil {
		return uint128Zero, uint128Zero, err
	}
	if opts.DefaultPrefix == 128 {
		return addr, addr, nil
	}
	lo := addr
	if opts.UseCIDRNetwork {
		lo = Network6(addr, opts.DefaultPrefix)
	}
	return lo, Broadcast6(lo, opts.DefaultPrefix), nil
}

func looksLikeIPv6(token string) bool {
	return strings.Contains(token, ":")
}

// splitRange6 recursively splits an IPv6 range into CIDR prefixes.
func splitRange6(addr Uint128, prefix int, lo, hi Uint128, prefixes [129]bool, emit func(Uint128, int) error) error {
	if prefix < 0 || prefix > 128 {
		return ErrInvalidPrefix
	}
	bc := Broadcast6(addr, prefix)
	if lo.LessThan(addr) || hi.GreaterThan(bc) {
		return fmt.Errorf("range %s-%s outside %s/%d", Uint128ToIPv6(lo), Uint128ToIPv6(hi), Uint128ToIPv6(addr), prefix)
	}
	if lo.Equals(addr) && hi.Equals(bc) && prefixes[prefix] {
		return emit(addr, prefix)
	}
	if prefix == 128 {
		return nil
	}

	nextPrefix := prefix + 1
	upperHalf := addr.SetBit(nextPrefix, true)
	if hi.LessThan(upperHalf) {
		return splitRange6(addr, nextPrefix, lo, hi, prefixes, emit)
	}
	if !lo.LessThan(upperHalf) {
		return splitRange6(upperHalf, nextPrefix, lo, hi, prefixes, emit)
	}
	if err := splitRange6(addr, nextPrefix, lo, Broadcast6(addr, nextPrefix), prefixes, emit); err != nil {
		return err
	}
	return splitRange6(upperHalf, nextPrefix, upperHalf, hi, prefixes, emit)
}
