package e2e

import (
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
	njui "github.com/lianxin255/jumpboard/internal/ui"
)

// The Phase 1 shell: the rail's width, the one status row every page
// that can apply draws, and the scene chips' weight of choice. These
// tests pin the redesign's geometry and its consolidation — one 应用配置,
// the engine's word on the page — not its colors, which the sidebar's
// own theme tests already pin.

// TestSidebarRailWidth checks the rail against its Phase 1 width: the
// sidebar's own color still reaches x=200, and the page's canvas starts
// by x=222, so the rail is 216 and not the 232 it was.
func TestSidebarRailWidth(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1280, 800)
	tt.Frame()
	img := tt.Image()
	want := njui.Dark()
	if got := img.RGBAAt(200, 300); !sameColor(got, want.Sidebar) {
		t.Errorf("the rail should still paint its color at x=200: got %v", got)
	}
	if got := img.RGBAAt(222, 300); !sameColor(got, want.Canvas) {
		t.Errorf("the canvas should start by x=222 (rail 216 + border): got %v", got)
	}
}

// TestEngineFooterMasterSwitch checks the rail's foot as the forwarding
// service's master control: the row names the service, its state and its
// engine, and the switch is the compact one — 40×22, not the 54×30 the
// theme's spacing drew — whole inside the rail, right of the row's words.
func TestEngineFooterMasterSwitch(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1280, 800)
	tt.Frame()

	sw, ok := tt.Find("转发服务开关")
	if !ok {
		t.Fatalf("the master switch is missing: %v", tt.Texts())
	}
	if d := sw.W - 40; d < -1 || d > 1 {
		t.Errorf("the master switch should be the compact 40 wide, got %.0f", sw.W)
	}
	if d := sw.H - 22; d < -1 || d > 1 {
		t.Errorf("the master switch should be the compact 22 tall, got %.0f", sw.H)
	}
	// The rail is 216 + its border; the switch sits inside it, right of
	// the service's name.
	if right := sw.X + sw.W; right > 217 {
		t.Errorf("the switch should sit inside the rail: right edge %.0f", right)
	}
	title, ok := tt.Find("转发服务")
	if !ok {
		t.Fatalf("the service's name is missing: %v", tt.Texts())
	}
	if sw.X < title.X+title.W {
		t.Errorf("the switch should sit right of the title: %.0f vs %.0f", sw.X, title.X+title.W)
	}
	// A fresh app's engine is running, and the foot says the state and
	// the engine it runs on; the stopped word arrives when it stops.
	if !tt.HasText("运行中") || !tt.HasText("内置引擎（Go）") {
		t.Errorf("the foot should say the service's state and engine: %v", tt.Texts())
	}
	// The window bar keeps its own, separate word — the status row the
	// header owns is not the control.
	if !tt.HasText("引擎 运行中") {
		t.Errorf("the window bar should keep its engine status: %v", tt.Texts())
	}
	if err := a.StopEngine(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the stopped words", func() bool {
		return tt.HasText("已停止") && tt.HasText("引擎 已停止")
	})
}

// TestMappingsToolbarSplitsCreationFromFilters checks the two tool rows'
// one meaning each: the scene chips, the search and the create actions
// share the first row; the status segments and the group chips hold the
// second. The rows may wrap — a narrow window moves a row's own tools to
// its next line — but a create action never lands on the filter row.
func TestMappingsToolbarSplitsCreationFromFilters(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"default 1280x800", 1280, 800},
		{"minimum 960x640", 960, 640},
	} {
		t.Run(size.name, func(t *testing.T) {
			a := testApp(t)
			if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
				Name: "分组站", Protocol: model.TCP, Enabled: true, ListenPort: freePort(t),
				Group: "数据库", UpstreamHost: "127.0.0.1", UpstreamPort: 1,
			}); err != nil {
				t.Fatal(err)
			}
			if err := a.Reload(); err != nil {
				t.Fatal(err)
			}
			tt := tester(t, a, size.width, size.height)
			waitFor(t, tt, "the row", func() bool { return tt.HasText("分组站") })

			creates := map[string]string{
				"从模板新建": "the template door",
				"粘贴导入":  "the paste door",
				"新建映射":  "the primary create",
				"场景":    "the scene chips",
			}
			filters := map[string]string{
				"筛选":    "the status segments",
				"组 数据库": "the group chips",
			}
			var createBottom float32
			for label, what := range creates {
				r, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%s is missing: %v", what, tt.Texts())
				}
				if r.W <= 0 || r.H <= 0 {
					t.Fatalf("%s has no geometry: %+v", what, r)
				}
				if b := r.Y + r.H; b > createBottom {
					createBottom = b
				}
			}
			for label, what := range filters {
				r, ok := tt.Find(label)
				if !ok {
					t.Fatalf("%s is missing: %v", what, tt.Texts())
				}
				if r.W <= 0 || r.H <= 0 {
					t.Fatalf("%s has no geometry: %+v", what, r)
				}
				// However the first row wrapped, every filter sits below
				// every create action: the meanings never share a line.
				if r.Y < createBottom-1 {
					t.Errorf("%s should sit below the create row: %.0f under %.0f", what, r.Y, createBottom)
				}
			}
			// On the wide window neither row needs to wrap: the create
			// actions share one line and the filters another.
			if size.width >= 1280 {
				rects := make([]float32, 0, len(creates))
				for label := range creates {
					r, _ := tt.Find(label)
					rects = append(rects, r.Y)
				}
				lo, hi := rects[0], rects[0]
				for _, y := range rects {
					if y < lo {
						lo = y
					}
					if y > hi {
						hi = y
					}
				}
				if hi-lo > 12 {
					t.Errorf("the create actions should share one line, %.0f DIPs apart", hi-lo)
				}
			}
		})
	}
}

// TestMappingEngineStatusOnTheWindowHeader checks the consolidation: the
// engine's state rides the window bar the mappings and history pages
// share, so the page states its engine without the eye going to the
// rail, and the bar's own words — 预检, 应用配置 — stay.
func TestMappingEngineStatusOnTheWindowHeader(t *testing.T) {
	a := testApp(t)
	listen := freePort(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "状态站", Protocol: model.TCP, Enabled: true, ListenPort: listen,
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Apply(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the row's engine word", func() bool { return tt.HasText("引擎 运行中") })
	if !tt.HasText("预检") || !tt.HasText("应用配置") {
		t.Fatalf("the row should keep its actions: %v", tt.Texts())
	}

	// The same row, same words, on the history page.
	if err := tt.Click("历史"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the history page's engine word", func() bool { return tt.HasText("引擎 运行中") })
	if !tt.HasText("预检") || !tt.HasText("应用配置") {
		t.Fatalf("the history page's row should keep its actions: %v", tt.Texts())
	}

	// A stopped engine says so in its place.
	if err := a.StopEngine(); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the stopped word", func() bool { return tt.HasText("引擎 已停止") })
}

// TestPendingBarHasOneApply checks that the amber bar states the pending
// change but no longer carries its own 应用配置: with a dirty
// configuration the whole window offers exactly one, the action row's.
func TestMappingPendingBarHasOneApply(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "待应用", Protocol: model.TCP, Enabled: true, ListenPort: freePort(t),
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the row's switch", func() bool { return tt.HasText("启用-待应用") })
	// A change made through the window is what leaves the configuration
	// dirty; the store write above only seeded the row.
	if err := tt.Click("启用-待应用"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the pending bar", func() bool { return tt.HasText("有未生效的改动") })
	if !tt.HasText("未应用") {
		t.Errorf("the bar must keep the word 未应用: %v", tt.Texts())
	}
	applies := 0
	for _, s := range tt.Texts() {
		if s == "应用配置" {
			applies++
		}
	}
	if applies != 1 {
		t.Errorf("the window should offer exactly one 应用配置, found %d: %v", applies, tt.Texts())
	}
	// The dirty word can arrive while the switch's own background work
	// still holds the window busy, and the apply button stays disabled
	// until that clears. Wait for the calm, then frame, so the click lands
	// on an enabled button.
	waitFor(t, tt, "the window calm", func() bool {
		_, _, _, _, _, _, busy, _ := a.Snapshot()
		return !busy
	})
	tt.Frame()
	// And the one button still applies.
	if err := tt.Click("应用配置"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the change applied", func() bool { return !a.Dirty() })
}

// TestSidebarActiveNavIsSolid checks the rail's current-page state: the
// active nav row carries the accent itself — solid, with the accent's own
// ink — while an inactive row stays transparent over the rail's color,
// the material's to show through.
func TestSidebarActiveNavIsSolid(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1280, 800)
	tt.Frame()
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the logs page", func() bool { return tt.HasText("操作日志") })
	img := tt.Image()
	tok := njui.Dark()
	// The nav item is the first 日志 on screen: the rail draws before the
	// window bar's own title says it again.
	nav, ok := tt.Find("日志")
	if !ok {
		t.Fatalf("the nav item is missing: %v", tt.Texts())
	}
	for _, p := range [][2]int{
		{int(nav.X - 5), int(nav.Y + nav.H/2)},         // the gap between icon and word
		{int(nav.X + nav.W + 6), int(nav.Y + nav.H/2)}, // the row's trailing fill
	} {
		if got := img.RGBAAt(p[0], p[1]); !sameColor(got, tok.Accent) {
			t.Errorf("the active nav row should carry the accent at %v: got %v", p, got)
		}
	}
	other, ok := tt.Find("历史")
	if !ok {
		t.Fatalf("an inactive nav item is missing: %v", tt.Texts())
	}
	if got := img.RGBAAt(int(other.X-5), int(other.Y+other.H/2)); !sameColor(got, tok.Sidebar) {
		t.Errorf("an inactive nav row should stay the rail's color: got %v", got)
	}
}

// TestWindowHeaderReplacesPageTitles pins the unified window bar: the
// count line the mappings page once drew under its own title is now the
// bar's subtitle — inside the bar's 56 DIPs, said exactly once — and the
// first thing below the bar is the page's toolbar, not a second heading.
func TestWindowHeaderReplacesPageTitles(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "表头站", Protocol: model.TCP, Enabled: true, ListenPort: freePort(t),
		UpstreamHost: "127.0.0.1", UpstreamPort: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1280, 800)
	waitFor(t, tt, "the row", func() bool { return tt.HasText("表头站") })

	sub := "1 条规则 · 1 启用 · 0 停用"
	subs := 0
	for _, s := range tt.Texts() {
		if s == sub {
			subs++
		}
	}
	if subs != 1 {
		t.Fatalf("the count line should be said once, in the bar: %d in %v", subs, tt.Texts())
	}
	r, ok := tt.Find(sub)
	if !ok {
		t.Fatalf("the bar's subtitle is missing: %v", tt.Texts())
	}
	if bottom := r.Y + r.H; bottom > 56+1 {
		t.Errorf("the subtitle should sit inside the 56 DIP bar: bottom %.0f", bottom)
	}
	// The toolbar, not a title row, is what follows: the scene caption
	// starts below the bar and the page's own padding.
	scene, ok := tt.Find("场景")
	if !ok {
		t.Fatalf("the scene chips are missing: %v", tt.Texts())
	}
	if scene.Y < 56+20-1 {
		t.Errorf("the toolbar should start below the bar: 场景 at %.0f", scene.Y)
	}

	// The logs page answers the same way: one subtitle, in the bar.
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the logs page", func() bool { return tt.HasText("操作日志") })
	subs = 0
	for _, s := range tt.Texts() {
		if s == "操作记录与内置引擎的访问日志" {
			subs++
		}
	}
	if subs != 1 {
		t.Errorf("the logs subtitle should be the bar's alone: %d in %v", subs, tt.Texts())
	}
}

// TestSceneChipHierarchy checks the chips' weight of choice: the active
// scene carries the accent itself, solid, while the scenes not in use
// stay the quiet glass the row's 管理 chip keeps.
func TestMappingSceneChipHierarchy(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateWorkspace("家里"); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "both scene chips", func() bool {
		return tt.HasText("场景 默认") && tt.HasText("场景 家里")
	})
	tt.Frame()
	img := tt.Image()
	tok := njui.Dark()
	on, ok := tt.Find("场景 默认")
	if !ok {
		t.Fatalf("the active chip is missing: %v", tt.Texts())
	}
	// Inside the chip's padding, clear of its text and corners.
	for _, p := range [][2]int{{int(on.X + 4), int(on.Y + on.H/2)}, {int(on.X + on.W - 4), int(on.Y + on.H/2)}} {
		if got := img.RGBAAt(p[0], p[1]); !sameColor(got, tok.Accent) {
			t.Errorf("the active scene chip should carry the accent at %v: got %v", p, got)
		}
	}
	quiet, ok := tt.Find("场景 家里")
	if !ok {
		t.Fatalf("the quiet chip is missing: %v", tt.Texts())
	}
	if got := img.RGBAAt(int(quiet.X+4), int(quiet.Y+quiet.H/2)); !sameColor(got, tok.Chip) {
		t.Errorf("an unused scene chip should stay glass: got %v", got)
	}
}
