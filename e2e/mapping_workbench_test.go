package e2e

import (
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// The Phase 1 mapping workbench: the forwarding relation as the row's
// hero, a health pill that never wraps, a pair of row actions that fits
// the window it is drawn in, and the default-port marker a web rule with
// a blank listen port carries.

// TestHealthPillNeverWraps checks the visual review's wrap at 1280x800:
// the ○ 未检查 pill stays one line — its height is a single caption's.
func TestHealthPillNeverWraps(t *testing.T) {
	a := testApp(t)
	for _, m := range []model.Mapping{
		{Name: "待检", Protocol: model.TCP, Enabled: true, ListenPort: freePort(t),
			UpstreamHost: "127.0.0.1", UpstreamPort: 1,
			HealthCheckEnabled: true, HealthStatus: model.HealthUnknown},
		{Name: "免检", Protocol: model.TCP, Enabled: true, ListenPort: freePort(t),
			UpstreamHost: "127.0.0.1", UpstreamPort: 1},
	} {
		if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), m); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1280, 800)
	waitFor(t, tt, "both rows", func() bool { return tt.HasText("待检") && tt.HasText("免检") })

	pill, ok := tt.Find("检查-待检")
	if !ok {
		t.Fatalf("the unchecked pill is missing: %v", tt.Texts())
	}
	// One caption line with its pill padding: a wrapped pill would be
	// twice that.
	if pill.H > 22 {
		t.Errorf("the unchecked pill wrapped: height %.0f, want one caption line", pill.H)
	}
	off, ok := tt.Find("未启用")
	if !ok {
		t.Fatalf("the off pill is missing: %v", tt.Texts())
	}
	if off.H > 22 {
		t.Errorf("the off pill wrapped: height %.0f", off.H)
	}
}

// TestMappingRowActionsFitWindow checks the workbench at the default and
// minimum window widths: the row's buttons sit whole inside the page, and
// the forwarding relation — the redesign's visible link — keeps its every
// part drawn. The stress rule carries the widest content a real rule
// books (a long ServerNames local, an 18-character upstream, a group); a
// normal-length rule beside it proves the relation shows whole, not
// merely occupying a box: an ellipsized text fills its box exactly to the
// column's edge, so a right edge that stops short of the edge is a whole
// one.
func TestMappingRowActionsFitWindow(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"default 1280x800", 1280, 800},
		{"minimum 960x640", 960, 640},
	} {
		t.Run(size.name, func(t *testing.T) {
			a := testApp(t)
			for _, m := range []model.Mapping{
				{Name: "宽行", Protocol: model.HTTP, Enabled: true, ListenPort: freePort(t),
					ServerNames: "wide.example.com", Group: "分组甲",
					UpstreamHost: "192.168.100.100", UpstreamPort: 8080,
					HealthCheckEnabled: true, HealthStatus: model.HealthUnknown},
				{Name: "常行", Protocol: model.TCP, Enabled: true, ListenPort: freePort(t),
					Group: "常组", UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
					HealthCheckEnabled: true, HealthStatus: model.HealthUnknown},
			} {
				if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), m); err != nil {
					t.Fatal(err)
				}
			}
			if err := a.Reload(); err != nil {
				t.Fatal(err)
			}
			tt := tester(t, a, size.width, size.height)
			waitFor(t, tt, "both rows", func() bool { return tt.HasText("宽行") && tt.HasText("常行") })

			edit, ok := tt.Find("编辑")
			if !ok {
				t.Fatalf("the edit button is missing: %v", tt.Texts())
			}
			del, ok := tt.Find("删除")
			if !ok {
				t.Fatalf("the delete button is missing: %v", tt.Texts())
			}
			// The page's right pad is 32; the table's last column ends there.
			if right := del.X + del.W; right > float32(size.width-32)+1 {
				t.Errorf("删除 is clipped: right edge %.0f past the table's %.0f", right, float32(size.width-32))
			}
			if right := edit.X + edit.W; right > del.X {
				t.Errorf("编辑 (%.0f) should sit left of 删除 (%.0f)", edit.X+edit.W, del.X)
			}
			// The row's switch is the compact one — 40×22, inside the
			// 64-wide 启用 column — not the 54×30 the theme's spacing drew,
			// which stood taller than the row it rode.
			sw, ok := tt.Find("启用-宽行")
			if !ok {
				t.Fatalf("the row's switch is missing: %v", tt.Texts())
			}
			if d := sw.W - 40; d < -1 || d > 1 {
				t.Errorf("the row's switch should be the compact 40 wide, got %.0f", sw.W)
			}
			if d := sw.H - 22; d < -1 || d > 1 {
				t.Errorf("the row's switch should be the compact 22 tall, got %.0f", sw.H)
			}
			// The pair is a pair: the clone button is gone from the column.
			if _, ok := tt.Find("克隆"); ok {
				t.Error("克隆 should live in the row's context menu, not the column")
			}

			// The forward column ends where the health column begins; the
			// health pill anchors its left edge.
			anchor, ok := tt.Find("检查-宽行")
			if !ok {
				t.Fatalf("the health pill is missing: %v", tt.Texts())
			}
			forwardRight := anchor.X - 8

			// Every part of the stress rule's relation is drawn — nonzero,
			// inside the column — at any width the window allows. The
			// stress row is the table's first, so the unlabeled arrow
			// found first is its own. The Relay connector carries the
			// row's name as its label, so it is found the way the
			// addresses are.
			for label, what := range map[string]string{
				"复制地址-宽行":              "the local address",
				"转发-宽行":                "the relay connector",
				"→":                    "the forwarding arrow",
				"192.168.100.100:8080": "the upstream",
				"分组 分组甲":               "the group chip",
			} {
				r, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%s is missing: %v", what, tt.Texts())
				}
				if r.W <= 0 || r.H <= 0 {
					t.Errorf("%s has no geometry at %dx%d: %+v", what, size.width, size.height, r)
				}
				if right := r.X + r.W; right > forwardRight {
					t.Errorf("%s (%.0f) runs past the forward column (%.0f)", what, right, forwardRight)
				}
			}
			// The normal rule's relation shows whole: its upstream and group
			// stop short of the column's edge, where an ellipsis would fill
			// the box exactly.
			for label, what := range map[string]string{
				"10.0.0.1:3306": "the normal rule's upstream",
				"分组 常组":         "the normal rule's group chip",
			} {
				r, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%s is missing: %v", what, tt.Texts())
				}
				if r.W <= 0 || r.H <= 0 {
					t.Errorf("%s has no geometry at %dx%d: %+v", what, size.width, size.height, r)
				}
				if right := r.X + r.W; right > forwardRight-4 {
					t.Errorf("%s should show whole (right %.0f should stop short of the column edge %.0f)", what, right, forwardRight)
				}
			}
			// The hero begins after the name column: no column eats another.
			if addr, ok := tt.Find("复制地址-宽行"); ok {
				if right := addr.X + addr.W; right > edit.X {
					t.Errorf("the forward cell (%.0f) runs into the actions column (%.0f)", right, edit.X)
				}
			}

			// The counts bar is the workbench panel's footer: below the
			// rows it counts, inside the page's own bottom padding.
			foot, ok := tt.Find("当前列表 2 条 · 启用 2 · 停用 0")
			if !ok {
				t.Fatalf("the batch footer is missing: %v", tt.Texts())
			}
			if foot.Y < anchor.Y+anchor.H {
				t.Errorf("the batch footer should sit below the rows: %.0f < %.0f", foot.Y, anchor.Y+anchor.H)
			}
			if bottom := foot.Y + foot.H; bottom > float32(size.height)-20+1 {
				t.Errorf("the batch footer should stay inside the page: bottom %.0f of %d", bottom, size.height)
			}
		})
	}
}

// TestDefaultPortMarker checks the marker a web rule with a blank listen
// port carries: the forward cell says the default it will take — 80, or
// 443 for HTTPS — with （默认） beside it, the detail popover says the
// same, and the copied address stays the plain port.
func TestMappingDefaultPortMarker(t *testing.T) {
	a := testApp(t)
	for _, m := range []model.Mapping{
		{Name: "默认站", Protocol: model.HTTP, Enabled: true,
			UpstreamHost: "127.0.0.1", UpstreamPort: 8080},
		{Name: "默认加密站", Protocol: model.HTTPS, Enabled: true,
			UpstreamHost: "127.0.0.1", UpstreamPort: 8443},
	} {
		if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), m); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1280, 800)
	waitFor(t, tt, "both rows", func() bool {
		return tt.HasText("默认站") && tt.HasText("默认加密站")
	})
	// Both web rows carry the marker beside their address chips.
	markers := 0
	for _, s := range tt.Texts() {
		if s == "（默认）" {
			markers++
		}
	}
	if markers != 2 {
		t.Errorf("both web rows should carry （默认）, found %d: %v", markers, tt.Texts())
	}
	// A rule with its own port carries none.
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "自选端口站", Protocol: model.HTTP, Enabled: true, ListenPort: freePort(t),
		UpstreamHost: "127.0.0.1", UpstreamPort: 8080,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the third row", func() bool { return tt.HasText("自选端口站") })
	markers = 0
	for _, s := range tt.Texts() {
		if s == "（默认）" {
			markers++
		}
	}
	if markers != 2 {
		t.Errorf("a rule with its own port should not carry the marker: %d remain", markers)
	}

	// The detail popovers say the synthesized port with the same marker —
	// the popover's line carries no label of its own, so its words are
	// the page's own texts. The name opens the popover; the row's
	// accessible label shares the name's text, so the click goes by
	// place: on the name, inside the 规则 cell.
	for _, probe := range []struct{ name, line string }{
		{"默认站", "127.0.0.1:80（默认）"},
		{"默认加密站", "127.0.0.1:443（默认）"},
	} {
		r, ok := tt.Find(probe.name)
		if !ok {
			t.Fatalf("the row %s is missing: %v", probe.name, tt.Texts())
		}
		tt.ClickAt(r.X+30, r.Y+22)
		tt.Frame()
		if !tt.HasText(probe.line) {
			t.Errorf("the popover for %s should say %q: %v", probe.name, probe.line, tt.Texts())
		}
		if err := tt.Click("关闭详情"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}

	// The copied address stays the rule's own words: the marker is a
	// label, not part of the address.
	if err := tt.Click("复制地址-默认站"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the address copied", func() bool { return a.Copied() != "" })
	if got := a.Copied(); got != "本机:80" {
		t.Errorf("copied %q, want 本机:80", got)
	}
	if err := tt.Click("复制地址-默认加密站"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the TLS address copied", func() bool { return a.Copied() == "本机:443" })
	if got := a.Copied(); got != "本机:443" {
		t.Errorf("copied %q, want 本机:443", got)
	}
}
