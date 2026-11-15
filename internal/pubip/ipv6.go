package pubip

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
)

// BuildIPv6 combines an IPv6 prefix (e.g. the LAN prefix delegated by a router)
// with an optional prefix/subnet ID and an interface ID into a full address.
//
// prefixLen (if > 0) sets the prefix length and overrides a length carried by
// prefix ("2001:db8:1200::/56"); otherwise the length from prefix is used.
// prefixID (hex, e.g. "bc") fills the bits between the prefix length and /64.
// interfaceID is the lower 64 bits, e.g. "66ec:face:7324:81ea".
func BuildIPv6(prefix string, prefixLen int, prefixID, interfaceID string) (netip.Addr, error) {
	prefix = strings.TrimSpace(prefix)
	addrPart, lenPart, hasLen := strings.Cut(prefix, "/")

	a, err := netip.ParseAddr(addrPart)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid IPv6 prefix %q: %w", prefix, err)
	}

	bits := prefixLen
	if bits <= 0 {
		if !hasLen {
			return netip.Addr{}, fmt.Errorf("IPv6 prefix %q has no length and IPv6_PREFIX_LENGTH is not set", prefix)
		}
		bits, err = strconv.Atoi(lenPart)
		if err != nil {
			return netip.Addr{}, fmt.Errorf("invalid IPv6 prefix %q: %w", prefix, err)
		}
	}

	p, err := a.Prefix(bits)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid IPv6 prefix length %d: %w", bits, err)
	}

	if !p.Addr().Is6() || p.Addr().Is4In6() {
		return netip.Addr{}, fmt.Errorf("prefix %q is not an IPv6 prefix", prefix)
	}
	if p.Bits() < 1 || p.Bits() > 64 {
		return netip.Addr{}, fmt.Errorf("IPv6 prefix length must be between 1 and 64, got %d", p.Bits())
	}

	b := p.Masked().Addr().As16()

	if id := strings.TrimSpace(prefixID); id != "" {
		v, err := strconv.ParseUint(id, 16, 64)
		if err != nil {
			return netip.Addr{}, fmt.Errorf("invalid IPv6 prefix ID %q (expected hex): %w", id, err)
		}
		subnetBits := 64 - p.Bits()
		if v >= uint64(1)<<subnetBits {
			return netip.Addr{}, fmt.Errorf("IPv6 prefix ID %q does not fit into the %d bits between /%d and /64", id, subnetBits, p.Bits())
		}
		upper := binary.BigEndian.Uint64(b[:8]) | v
		binary.BigEndian.PutUint64(b[:8], upper)
	}

	iid, err := parseInterfaceID(interfaceID)
	if err != nil {
		return netip.Addr{}, err
	}
	copy(b[8:], iid[8:])

	return netip.AddrFrom16(b), nil
}

func parseInterfaceID(id string) ([16]byte, error) {
	s := strings.TrimSpace(id)
	if !strings.HasPrefix(s, "::") {
		s = "::" + s
	}

	a, err := netip.ParseAddr(s)
	if err != nil || !a.Is6() {
		return [16]byte{}, fmt.Errorf("invalid IPv6 interface ID %q", id)
	}

	b := a.As16()
	if binary.BigEndian.Uint64(b[:8]) != 0 {
		return [16]byte{}, fmt.Errorf("IPv6 interface ID %q is longer than 64 bits", id)
	}

	return b, nil
}
