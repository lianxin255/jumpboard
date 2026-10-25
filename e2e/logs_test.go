package e2e

import (
	"testing"
)

// TestLogFilters checks the log page's two filters: the level picks one
// kind of line, and the search keeps only what matches.
func TestLogFilters(t *testing.T) {
	a := testApp(t)
	a.Log.Info("同步完成-苹果42")
	a.Log.Warn("磁盘将满-香蕉7")
	a.Log.Error("应用失败-樱桃99")
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	a.Change()

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	for _, want := range []string{"苹果42", "香蕉7", "樱桃99"} {
		if !tt.HasText(want) {
			t.Errorf("%q should be listed: %v", want, tt.Texts())
		}
	}

	// The level filter narrows the list to exactly that level.
	for _, tc := range []struct {
		level string
		yes   string
		no    []string
	}{
		{"错误", "樱桃99", []string{"苹果42", "香蕉7"}},
		{"警告", "香蕉7", []string{"苹果42", "樱桃99"}},
		{"信息", "苹果42", []string{"香蕉7", "樱桃99"}},
	} {
		if err := tt.Click(tc.level); err != nil {
			t.Fatalf("clicking the %q level: %v", tc.level, err)
		}
		tt.Frame()
		if !tt.HasText(tc.yes) {
			t.Errorf("level %q should keep %q: %v", tc.level, tc.yes, tt.Texts())
		}
		for _, no := range tc.no {
			if tt.HasText(no) {
				t.Errorf("level %q should hide %q: %v", tc.level, no, tt.Texts())
			}
		}
	}

	// Back to everything, the search keeps the matching line only.
	if err := tt.Click("全部"); err != nil {
		t.Fatal(err)
	}
	fillField(t, tt, "搜索日志", "苹果")
	tt.Frame()
	if !tt.HasText("苹果42") {
		t.Errorf("the search should keep the match: %v", tt.Texts())
	}
	for _, no := range []string{"香蕉7", "樱桃99"} {
		if tt.HasText(no) {
			t.Errorf("the search should hide %q: %v", no, tt.Texts())
		}
	}

	fillField(t, tt, "搜索日志", "什么都不匹配的词")
	tt.Frame()
	if !tt.HasText("没有日志") {
		t.Errorf("no match should show the empty state: %v", tt.Texts())
	}
}
