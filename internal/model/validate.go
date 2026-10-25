package model

import (
	"regexp"
	"strconv"
	"strings"
)

// ValidPort reports whether p is a usable TCP/UDP port.
func ValidPort(p int) bool { return p >= 1 && p <= 65535 }

// ClampInt limits n to [lo, hi].
func ClampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

var leadingZero = regexp.MustCompile(`^(0|[1-9][0-9]{0,2})$`)

// ValidIPOrCIDR reports whether s is an IPv4 address or an IPv4 CIDR block.
// Leading zeros are rejected, and so is 0.0.0.0/0, which would match every
// address and silently turn the whitelist off.
func ValidIPOrCIDR(s string) bool {
	ipPart, cidr, hasCIDR := strings.Cut(s, "/")
	segs := strings.Split(ipPart, ".")
	if len(segs) != 4 {
		return false
	}
	for _, seg := range segs {
		if !leadingZero.MatchString(seg) {
			return false
		}
		n, err := strconv.Atoi(seg)
		if err != nil || n < 0 || n > 255 {
			return false
		}
	}
	if !hasCIDR {
		return true
	}
	if cidr == "" {
		return false
	}
	for _, r := range cidr {
		if r < '0' || r > '9' {
			return false
		}
	}
	bits, err := strconv.Atoi(cidr)
	if err != nil || bits < 0 || bits > 32 {
		return false
	}
	return bits != 0
}

var (
	injectChars = strings.NewReplacer(";", "", "{", "", "}", "", "#", "", "\n", "", "\r", "")
	proxyInject = regexp.MustCompile(`[;{}#\r\n]`)
)

// StripInject removes the characters that would end the directive a value
// is written into, or escape its block: ; { } # and newlines.
func StripInject(s string) string {
	return strings.TrimSpace(injectChars.Replace(s))
}

// ValidProxyPassURL reports whether s may be written after proxy_pass: an
// http or https URL with no character that could end or escape the
// directive.
func ValidProxyPassURL(s string) bool {
	if s == "" || len(s) > 500 {
		return false
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		return false
	}
	return !proxyInject.MatchString(s)
}

// StripProxyHeaders removes the characters that could escape a location
// block from user-written header directives, keeping the semicolons that
// end the directives.
func StripProxyHeaders(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '{', '}', '#', '\r':
			continue
		case '\n':
			b.WriteRune('\n')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
