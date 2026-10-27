package engine

import (
	"strconv"
	"testing"
	"time"
)

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

// waitActive polls a rule's active count until it reaches want, so a test
// sees a counter settle without hanging when it never does: the decrement
// runs in the goroutine that served the connection, which the client can
// outrun by finishing its read first.
func waitActive(t *testing.T, e *Local, mappingID string, want int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	var last int
	for {
		found := false
		for _, r := range e.Rules() {
			if r.MappingID != mappingID {
				continue
			}
			found, last = true, r.Active
			if r.Active == want {
				return
			}
		}
		if !found {
			t.Fatalf("rule %q not found", mappingID)
		}
		if time.Now().After(deadline) {
			t.Fatalf("rule %q active = %d, want %d (waited %v)", mappingID, last, want, within)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
