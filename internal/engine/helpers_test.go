package engine

import "strconv"

// itoa renders a port for the tests' URLs.
func itoa(n int) string { return strconv.Itoa(n) }

// atoiOr parses a port, returning def when it does not parse.
func atoiOr(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
