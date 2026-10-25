package engine

import (
	"bufio"
	"net"
)

// bufio_ReadWriter is the type http.Hijacker returns, aliased so the
// recorder can implement it without importing bufio under its own name.
type bufio_ReadWriter = bufio.ReadWriter

// whitelist.go: the address filter a mapping applies.

// parseWhitelist turns IP and CIDR strings into networks.
//
// An address without a mask is a /32. Anything unparsable is dropped: the
// store validates what the user types, and a value that got here anyway
// must not widen the whitelist.
func parseWhitelist(entries []string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(entries))
	for _, e := range entries {
		if _, n, err := net.ParseCIDR(e); err == nil {
			out = append(out, n)
			continue
		}
		ip := net.ParseIP(e)
		if ip == nil {
			continue
		}
		bits := 32
		if ip.To4() == nil {
			bits = 128
		}
		out = append(out, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return out
}

// allowed reports whether an address passes the whitelist.
//
// An empty whitelist allows everyone: the app refuses to apply with an
// empty whitelist turned on, so an empty list here means there is none.
func allowed(list []*net.IPNet, remoteAddr string) bool {
	if len(list) == 0 {
		return true
	}
	host := clientIP(remoteAddr)
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, n := range list {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
