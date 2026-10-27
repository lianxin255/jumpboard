package ui

import "testing"

// TestFormatBytes checks the byte rendering the mappings table shows next
// to each rule.
func TestFormatBytes(t *testing.T) {
	for _, c := range []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1230, "1.2 KB"},
		{1024 * 1024, "1.0 MB"},
		{3565158, "3.4 MB"},
		{5 << 30, "5.0 GB"},
		{2 << 40, "2.0 TB"},
	} {
		if got := formatBytes(c.n); got != c.want {
			t.Errorf("formatBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
