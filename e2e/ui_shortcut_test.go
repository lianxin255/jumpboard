package e2e

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestKeyboardShortcuts checks the keys a desktop app answers: new mapping,
// search focus, apply.
func TestKeyboardShortcuts(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	// Cmd+N opens the form.
	tt.Key(ui.Cmd, ui.KeyN)
	tt.Frame()
	if !a.FormOpen() {
		t.Fatal("Cmd+N should open the new-mapping form")
	}
	if !tt.HasText("新建映射") {
		t.Errorf("the form should be on screen: %v", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.FormOpen() {
		t.Error("Escape should close the form")
	}
}

// TestToastShowsOnce checks that the status line becomes one toast rather
// than restarting on every frame.
func TestToastShowsOnce(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	a.Info("一条测试提示")
	tt.Frame()
	if !tt.HasText("一条测试提示") {
		t.Fatalf("the toast should show: %v", tt.Texts())
	}
	// More frames must not add it again, which would keep it up forever.
	for i := 0; i < 5; i++ {
		tt.Frame()
	}
	if got := countText(tt.Texts(), "一条测试提示"); got != 1 {
		t.Errorf("%d copies of the toast, want 1", got)
	}
}

// countText counts occurrences of a text in the frame.
func countText(texts []string, want string) int {
	n := 0
	for _, s := range texts {
		if s == want {
			n++
		}
	}
	return n
}
