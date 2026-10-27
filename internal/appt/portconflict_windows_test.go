//go:build windows

package appt

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// TestParseNetstatANO pins the rows that count: a LISTENING TCP row on the
// port yields its pid once, other ports and other states do not, and a
// UDP row (no state column) yields the pid in its place.
func TestParseNetstatANO(t *testing.T) {
	out := strings.Join([]string{
		"  TCP    0.0.0.0:9000           0.0.0.0:0              LISTENING       111",
		"  TCP    127.0.0.1:9000         127.0.0.1:51000        ESTABLISHED     222",
		"  TCP    0.0.0.0:19000          0.0.0.0:0              LISTENING       333",
		"  UDP    0.0.0.0:9000           *:*                                    444",
		"", // netstat ends with a blank line
	}, "\r\n")

	if got := parseNetstatANO(out, 9000, "TCP"); len(got) != 1 || got[0] != "111" {
		t.Fatalf("parseNetstatANO(9000, TCP) = %v, want [111]: a listening row and no other", got)
	}
	if got := parseNetstatANO(out, 9000, "UDP"); len(got) != 1 || got[0] != "444" {
		t.Fatalf("parseNetstatANO(9000, UDP) = %v, want [444]", got)
	}
	if got := parseNetstatANO(out, 19000, "TCP"); len(got) != 1 || got[0] != "333" {
		t.Fatalf("parseNetstatANO(19000, TCP) = %v, want [333]: :9000 is not :19000", got)
	}
}

// TestTasklistNames pairs the CSV's image with its pid, and leaves a pid
// the table has no row for unnamed.
func TestTasklistNames(t *testing.T) {
	restore := stubbedTasklist("\"nginx.exe\",\"111\",\"Console\",\"1\",\"1,024 K\"\r\n\"chrome.exe\",\"222\",\"Console\",\"1\",\"99,999 K\"\r\n")
	t.Cleanup(restore)

	names := tasklistNames([]string{"111", "333"})
	if names["111"] != "nginx.exe" {
		t.Fatalf("tasklistNames(111) = %q, want nginx.exe", names["111"])
	}
	if names["333"] != "" {
		t.Fatalf("a pid with no row should stay unnamed, got %q", names["333"])
	}
}

// stubbedTasklist answers tasklist with out, and errors for any other
// tool, so a test of the name lookup reads only the table it wrote.
func stubbedTasklist(out string) func() {
	return withToolExecuter(func(ctx context.Context, name string, args []string) ([]byte, error) {
		if name == "tasklist" {
			return []byte(out), nil
		}
		return nil, errors.New("not this tool")
	})
}
