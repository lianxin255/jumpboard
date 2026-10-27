package e2e

import (
	"strings"
	"testing"
)

// TestDirectiveChips checks the form's quick inserts: a click appends the
// directive to the box, and a second click does not double it.
func TestDirectiveChips(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("新建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	reveal(t, tt, "超时 1 小时")
	if err := tt.Click("超时 1 小时"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	_, _, f, _ := a.FormState()
	if !strings.Contains(f.ExtraDirectives, "proxy_timeout 1h;") {
		t.Fatalf("the chip should insert the directive: %q", f.ExtraDirectives)
	}

	// A second click is a no-op: the box already speaks proxy_timeout.
	if err := tt.Click("超时 1 小时"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	_, _, f2, _ := a.FormState()
	if n := strings.Count(f2.ExtraDirectives, "proxy_timeout"); n != 1 {
		t.Errorf("the directive was doubled: %q", f2.ExtraDirectives)
	}

	// A different directive still appends, on its own line.
	if err := tt.Click("不缓冲"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	_, _, f3, _ := a.FormState()
	if !strings.Contains(f3.ExtraDirectives, "proxy_buffering off;") {
		t.Errorf("the second chip should append: %q", f3.ExtraDirectives)
	}
	if !strings.Contains(f3.ExtraDirectives, "proxy_timeout") {
		t.Errorf("the first directive should survive: %q", f3.ExtraDirectives)
	}
}

// TestFormCloseButton checks the × in the corner: it closes the form the
// same way 取消 does, without saving.
func TestFormCloseButton(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("新建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	fillField(t, tt, "名称", "没保存的")
	if err := tt.Click("关闭"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.FormOpen() {
		t.Error("the × should close the form")
	}
	if list, _ := a.Store.ListMappings(a.ActiveWorkspace()); len(list) != 0 {
		t.Errorf("closing saves nothing: %+v", list)
	}
}
