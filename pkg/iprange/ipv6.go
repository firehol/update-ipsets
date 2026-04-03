package iprange

// IPv6Set is kept as a compatibility alias for callers that used the old
// placeholder type name while the Go package was still IPv4-only.
type IPv6Set = IPSet6

func SupportsIPv6() bool {
	return true
}
