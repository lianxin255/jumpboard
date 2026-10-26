package ui

import "testing"

// TestStripConfTimestamp checks that the generated header's timestamp is
// blanked and everything else — including the line it sits on — stays.
func TestStripConfTimestamp(t *testing.T) {
	in := "# 由 JumpBoard 自动生成，请勿手动编辑\n# 生成时间: 2026-10-26T10:00:00.000Z\nworker_processes 1;\n"
	got := stripConfTimestamp(in)
	want := "# 由 JumpBoard 自动生成，请勿手动编辑\n# 生成时间: \nworker_processes 1;\n"
	if got != want {
		t.Fatalf("stripConfTimestamp:\n got %q\nwant %q", got, want)
	}
	// A configuration without the line is returned unchanged.
	if plain := "server { }\n"; stripConfTimestamp(plain) != plain {
		t.Errorf("a conf without a timestamp should pass through")
	}
}

// TestSnapshotDiffStats checks the numbers the dialog shows: two identical
// rule sets differ by nothing once the timestamp is blanked, and a snapshot
// that adds a block counts the lines it adds.
func TestSnapshotDiffStats(t *testing.T) {
	base := "# 生成时间: 2026-10-26T10:00:00.000Z\nserver {\n  listen 9000;\n}\n"
	later := "# 生成时间: 2026-10-26T11:30:45.123Z\nserver {\n  listen 9000;\n}\n"
	if added, deleted := snapshotDiffStats(base, later); added != 0 || deleted != 0 {
		t.Errorf("only the timestamp differs, got +%d −%d", added, deleted)
	}

	withExtra := later + "server {\n  listen 9001;\n}\n"
	added, deleted := snapshotDiffStats(base, withExtra)
	if added != 3 || deleted != 0 {
		t.Errorf("the added block is 3 lines, got +%d −%d", added, deleted)
	}
}
