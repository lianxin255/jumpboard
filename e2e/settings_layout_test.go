package e2e

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The settings page's shape, at the two window sizes the design names:
// 1280×800 sits the cards in rows of two, top aligned and equal in
// width, with the AI card alone on a full-width row of its own; 960×640
// stacks every card in one full-width column, and the page still scrolls
// to what sits at its bottom. A card below the fold has no box, so each
// measurement scrolls the pair into view first, the way a user reaches
// it.

// settingsCardRects scrolls the settings page until every named card
// shows together — a card below the fold has no box — and returns the
// boxes of the cards' title texts, which stretch to the cards' width.
func settingsCardRects(t *testing.T, tt *ui.Tester, names ...string) []ui.Rect {
	t.Helper()
	rects := make([]ui.Rect, len(names))
	for i := 0; i < 40; i++ {
		ok := true
		for j, name := range names {
			r, found := tt.Find(name)
			rects[j] = r
			ok = ok && found && r.W > 0
		}
		if ok {
			return rects
		}
		tt.Scroll(600, 400, 0, 120)
		tt.Frame()
	}
	t.Fatalf("the cards %v never showed together", names)
	return nil
}

// settingsNoOverflow holds a card inside the page: its title ends before
// the page's right padding, which ends pageRight DIPs from the window's
// edge.
func settingsNoOverflow(t *testing.T, name string, r ui.Rect, pageRight float32) {
	t.Helper()
	if r.X < 200 {
		t.Errorf("%s starts at %v, left of the content column", name, r.X)
	}
	if right := r.X + r.W; right > pageRight {
		t.Errorf("%s ends at %v, past the page's right edge %v", name, right, pageRight)
	}
}

// TestSettingsCardsPairAt1280 checks the wide shape: the AI card on a
// row of its own, then rows of two cards — top aligned, equal in width,
// nothing three abreast, nothing past the page's right edge — and the AI
// card's form beside its reading matter rather than above it.
func TestSettingsCardsPairAt1280(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1280, 800)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// The page's cards stop 32 DIPs before the window's edge; a card's
	// title, inside the card's padding and border, a little sooner.
	const pageRight = 1280 - 32

	// The AI card leads the page and owns its row whole.
	ai := settingsCardRects(t, tt, "AI 配置")[0]
	settingsNoOverflow(t, "AI 配置", ai, pageRight-16)

	// Its form and its reading matter sit side by side: the privacy
	// heading is beside the endpoint field, not below it.
	rects := settingsCardRects(t, tt, "接口地址", "数据与隐私")
	form, notes := rects[0], rects[1]
	settingsNoOverflow(t, "数据与隐私", notes, pageRight-16)
	if notes.X < form.X+200 {
		t.Errorf("the notes should sit beside the form, at %v and %v", form.X, notes.X)
	}
	if d := notes.Y - form.Y; d < -1 || d > 120 {
		t.Errorf("the notes should start level with the form, %v DIPs apart", d)
	}

	// Every pair: two cards, top aligned, equal in width. The first pair
	// shows on the opening frame; the pairs below it are scrolled to,
	// which is why its width is kept now — a card scrolled past cannot be
	// scrolled back to by a page that only scrolls down.
	pairW := float32(0)
	for _, pair := range [][2]string{
		{"外观与显示", "通用"},
		{"访问控制", "备份与快照"},
		{"场景", "快捷键"},
	} {
		rects := settingsCardRects(t, tt, pair[0], pair[1])
		left, right := rects[0], rects[1]
		pairW = left.W
		settingsNoOverflow(t, pair[0], left, pageRight-16)
		settingsNoOverflow(t, pair[1], right, pageRight-16)
		if d := right.Y - left.Y; d < -0.5 || d > 0.5 {
			t.Errorf("%s and %s should start level, %v DIPs apart", pair[0], pair[1], d)
		}
		if d := left.W - right.W; d < -1 || d > 1 {
			t.Errorf("%s and %s should be equal in width, %v vs %v", pair[0], pair[1], left.W, right.W)
		}
		if right.X < left.X+left.W+16 {
			t.Errorf("%s should sit beside %s with the row's gap, at %v and %v", pair[1], pair[0], left.X, right.X)
		}
		// The switches are the compact ones — measured while the pair is
		// in view, before the page scrolls on: 40×22, not the 54×30 the
		// theme's spacing drew, a control half as wide again as the lines
		// it rides. The login-item switch rides the 通用 pair.
		if pair[1] == "通用" {
			sw, ok := tt.Find("开机自启动")
			if !ok {
				t.Fatalf("the login-item switch is missing: %v", tt.Texts())
			}
			if d := sw.W - 40; d < -1 || d > 1 {
				t.Errorf("the switch should be the compact 40 wide, got %.0f", sw.W)
			}
			if d := sw.H - 22; d < -1 || d > 1 {
				t.Errorf("the switch should be the compact 22 tall, got %.0f", sw.H)
			}
		}
	}

	// 更新 has no partner: alone on its row, it spans the page the way
	// the AI card does, below the last pair. Its place is measured on one
	// frame with the pair above it — rects from different scrolls sit at
	// different window offsets and cannot be compared.
	rects = settingsCardRects(t, tt, "快捷键", "更新")
	hotkey, upd := rects[0], rects[1]
	settingsNoOverflow(t, "更新", upd, pageRight-16)
	if d := upd.W - ai.W; d < -1 || d > 1 {
		t.Errorf("the lone 更新 card should span the page like the AI card: %v vs %v", upd.W, ai.W)
	}
	if upd.Y < hotkey.Y {
		t.Errorf("更新 should sit below the pairs, at %v under %v", upd.Y, hotkey.Y)
	}

	// The AI card is wider than a pair's two cards side by side would
	// allow one of them: it spans the page while they split it.
	if ai.W < pairW*1.5 {
		t.Errorf("the AI card should span its own full-width row: %v vs a card's %v", ai.W, pairW)
	}

	// The privacy lines stay whole on the card, neither folded away nor
	// trimmed for the two-column shape. The interpretation line tells the
	// two paths apart: a file over the cap is refused, the access log
	// sends only its tail.
	for _, line := range []string{
		"Key 存在系统凭据库",
		"超过 256 KB 会被拒绝",
		"访问日志只发送末尾 200 行",
		"映射摘要里永远不含密钥",
	} {
		if !tt.HasText(line) {
			t.Errorf("the privacy notes should keep %q", line)
		}
	}
}

// TestSettingsCardsStackAt960 checks the narrow shape: one full-width
// column — each pair's second card below its first, everything the same
// width, the AI card's notes below its form — and the page still scrolls
// to the danger band at its bottom.
func TestSettingsCardsStackAt960(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 960, 640)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	const pageRight = 960 - 32

	// The AI card's reading matter sits below its form, the column the
	// narrow window leaves the card.
	rects := settingsCardRects(t, tt, "接口地址", "数据与隐私")
	form, notes := rects[0], rects[1]
	settingsNoOverflow(t, "数据与隐私", notes, pageRight-16)
	if d := notes.X - form.X; d < -0.5 || d > 0.5 {
		t.Errorf("the notes should stack under the form's edge, %v vs %v", form.X, notes.X)
	}
	if notes.Y < form.Y+150 {
		t.Errorf("the notes should sit below the form, at %v and %v", form.Y, notes.Y)
	}

	ai := settingsCardRects(t, tt, "AI 配置")[0]

	// Every pair stacks: the second card below the first, both the width
	// of the page, which is also the AI card's width.
	for _, p := range [][2]string{
		{"外观与显示", "通用"},
		{"访问控制", "备份与快照"},
		{"场景", "快捷键"},
	} {
		rects := settingsCardRects(t, tt, p[0], p[1])
		left, right := rects[0], rects[1]
		settingsNoOverflow(t, p[0], left, pageRight-16)
		settingsNoOverflow(t, p[1], right, pageRight-16)
		if d := right.X - left.X; d < -0.5 || d > 0.5 {
			t.Errorf("%s should stack at %s's edge, %v vs %v", p[1], p[0], left.X, right.X)
		}
		if right.Y < left.Y+left.H {
			t.Errorf("%s should sit below %s, at %v and %v", p[1], p[0], left.Y, right.Y)
		}
		if d := left.W - ai.W; d < -1 || d > 1 {
			t.Errorf("%s should fill the page's width like the AI card, %v vs %v", p[0], left.W, ai.W)
		}
	}

	// 更新 alone stacks with them: the page's own width, below the pairs,
	// measured on one frame with the pair above it — rects from different
	// scrolls sit at different window offsets and cannot be compared.
	rects = settingsCardRects(t, tt, "快捷键", "更新")
	hotkey, upd := rects[0], rects[1]
	settingsNoOverflow(t, "更新", upd, pageRight-16)
	if d := upd.W - ai.W; d < -1 || d > 1 {
		t.Errorf("the lone 更新 card should fill the page's width like the AI card, %v vs %v", upd.W, ai.W)
	}
	if upd.Y < hotkey.Y {
		t.Errorf("更新 should sit below the pairs, at %v under %v", upd.Y, hotkey.Y)
	}

	// The bottom of the page still answers: the danger band scrolls into
	// view and its button opens the confirmation it must ask first.
	reveal(t, tt, "危险操作")
	if err := tt.Click("清空所有数据"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ConfirmState() == nil {
		t.Fatalf("the wipe should ask first: %v", tt.Texts())
	}
	if err := tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.ConfirmState() != nil {
		t.Error("canceling should close the confirmation")
	}
}
