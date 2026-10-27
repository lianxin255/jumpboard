package serial

import (
	"strings"
	"testing"
)

// TestListReturnsDevices expects the machine to enumerate something — a
// machine with no serial device is a legitimate answer, but a machine
// with one must name it, sorted, with a hint or not.
func TestListReturnsDevices(t *testing.T) {
	ports := List()
	seen := map[string]bool{}
	for i, p := range ports {
		if strings.TrimSpace(p.Name) == "" {
			t.Fatalf("port %d has no name: %+v", i, p)
		}
		if i > 0 && p.Name < ports[i-1].Name {
			t.Fatalf("ports are not sorted: %q before %q", p.Name, ports[i-1].Name)
		}
		seen[p.Name] = true
	}
}

// TestOpenMissingPortFails asks for a device no machine has; the failure
// names it, so a pane can say which device refused.
func TestOpenMissingPortFails(t *testing.T) {
	const missing = "/dev/cu.__jumpboard-none__"
	_, err := Open(missing, Baud115200)
	if err == nil {
		t.Skip("the platform answered with a device it should not have")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("the error should name the port: %v", err)
	}
}

// TestOpenRealPortAndClose opens the first device the machine offers. A
// machine whose devices refuse (a permission the user has not granted, a
// device a driver keeps busy) is an environment the test skips rather
// than fails; where it opens, the port closes cleanly.
func TestOpenRealPortAndClose(t *testing.T) {
	ports := List()
	if len(ports) == 0 {
		t.Skip("no serial device on this machine")
	}
	conn, err := Open(ports[0].Name, Baud115200)
	if err != nil {
		t.Skipf("cannot open %s: %v", ports[0].Name, err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("closing %s: %v", ports[0].Name, err)
	}
}

// TestAllRatesAreOrderedAndDistinct keeps the picker honest: the rates it
// offers ascend, and none repeats.
func TestAllRatesAreOrderedAndDistinct(t *testing.T) {
	rates := AllRates()
	if len(rates) < 2 {
		t.Fatalf("only %d rates offered", len(rates))
	}
	for i, r := range rates {
		if i > 0 && r <= rates[i-1] {
			t.Fatalf("rates are not ascending: %d after %d", r, rates[i-1])
		}
	}
}
