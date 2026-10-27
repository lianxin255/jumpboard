//go:build !windows

package appt

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubbedTool answers with out when the command line names want, and an
// error otherwise, so a probe of one port cannot read another's answer.
func stubbedTool(want, out string) func() {
	return withToolExecuter(func(ctx context.Context, name string, args []string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), want) {
			return []byte(out), nil
		}
		return nil, errors.New("not this port")
	})
}

// TestParseLSOF pins the pairing of lsof -F's records: p before its c, one
// entry per process in first-seen order, the per-descriptor repeats gone.
func TestParseLSOF(t *testing.T) {
	got := parseLSOF("p123\ncnginx\nf11\nf12\np456\ncGoogle Chrome\nf9\np123\ncnginx\n")
	want := []procEntry{{PID: "123", Name: "nginx"}, {PID: "456", Name: "Google Chrome"}}
	if len(got) != len(want) {
		t.Fatalf("parseLSOF = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseLSOF[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
	if parseLSOF("") != nil && len(parseLSOF("")) != 0 {
		t.Errorf("empty stream should parse to no entries")
	}
	// A c with no p in front of it is skipped, not paired with a stale pid.
	if got := parseLSOF("corphan\n"); len(got) != 0 {
		t.Errorf("orphan c = %+v, want none", got)
	}
}

// TestWhoHolds reads the machine-readable stream through the injected
// tool: entries come back paired and deduped, and a port the tool cannot
// answer for reads as "cannot say" rather than an error.
func TestWhoHolds(t *testing.T) {
	restore := stubbedTool("TCP:9000", "p444\ncnginx\np444\ncnginx\nf11\n")
	t.Cleanup(restore)

	got := whoHolds(9000, "TCP")
	if len(got) != 1 || got[0].PID != "444" || got[0].Name != "nginx" {
		t.Fatalf("whoHolds(9000) = %+v, want [{444 nginx}]", got)
	}

	if got := whoHolds(9001, "TCP"); got != nil {
		t.Fatalf("a port the tool cannot answer for = %+v, want nil", got)
	}
}
