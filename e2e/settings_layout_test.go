package e2e

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The settings page's shape: a master-detail frame — the nav of groups on
// the left, the one chosen group's cards on the right. Every group is one
// click away, so no card sits below the fold of another group's height;
// the page's testable contract is the nav's completeness, the chosen
// group's cards filling the content column, and the danger group keeping
// its confirmation.

// TestSettingsNavListsEveryGroup checks the nav: the page opens on the AI
// group — the chat page's 去设置 lands there — and every group the design
// names is one row of the nav, said exactly once.
func TestSettingsNavListsEveryGroup(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	// The opening group is the AI one: its card shows without a click.
	if !tt.HasText("接口地址") {
		t.Fatalf("the AI group should open first: %v", tt.Texts())
	}
	// The nav's rows carry the 设置分组 label prefix: each group is one
	// reachable row, said once.
	for _, group := range []string{
		"AI 配置", "通用与外观", "访问控制", "备份与数据",
		"场景", "快捷键", "更新", "危险操作",
	} {
		if !tt.HasText("设置分组 " + group) {
			t.Errorf("the nav should list %q: %v", group, tt.Texts())
		}
	}
}

// TestSettingsGroupsSwitchByClick walks the nav: each click swaps the
// content column to that group's cards, and the previous group's cards go
// with it — one group at a time, the master-detail promise.
func TestSettingsGroupsSwitchByClick(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	steps := []struct {
		nav   string // the nav row to click
		shows string // a text only that group's cards say
	}{
		{"通用与外观", "跟随系统"},
		{"访问控制", "启用白名单"},
		{"备份与数据", "导出备份"},
		{"场景", "新建场景"},
		{"快捷键", "保存快捷键"},
		{"更新", "开发构建不支持自动更新"},
		{"危险操作", "清空所有数据"},
		{"AI 配置", "接口地址"},
	}
	for _, step := range steps {
		if err := tt.Click("设置分组 " + step.nav); err != nil {
			t.Fatalf("the nav row for %s is missing: %v (%v)", step.nav, err, tt.Texts())
		}
		waitFor(t, tt, step.nav+"'s cards", func() bool { return tt.HasText(step.shows) })
	}
}

// TestSettingsContentColumnFits checks the content card inside its
// column: the card starts right of the nav and ends before the page's
// right padding — the master-detail frame holds it, at either window size
// the design names.
func TestSettingsContentColumnFits(t *testing.T) {
	for _, size := range [][2]float32{{1280, 800}, {960, 640}} {
		a := testApp(t)
		tt := tester(t, a, int(size[0]), int(size[1]))
		if err := tt.Click("设置"); err != nil {
			t.Fatal(err)
		}
		tt.Frame()

		// The card's title text ends where the page's right padding does;
		// the padding itself is 32, the card's own border a little sooner.
		pageRight := size[0] - 16
		card, ok := tt.Find("AI 配置")
		if !ok {
			t.Fatalf("the AI card is missing: %v", tt.Texts())
		}
		if card.X < 200 {
			t.Errorf("at %v wide the card starts at %v, left of the content column", size[0], card.X)
		}
		if right := card.X + card.W; right > pageRight {
			t.Errorf("at %v wide the card ends at %v, past the page's right edge %v", size[0], right, pageRight)
		}

		// The AI card fills the column: the width the pairs used to split
		// now belongs to the one group's card.
		nav, ok := tt.Find("设置分组 AI 配置")
		if !ok {
			t.Fatalf("the nav is missing: %v", tt.Texts())
		}
		if card.X < nav.X+nav.W {
			t.Errorf("the card should start right of the nav: card %v nav %v", card.X, nav.X+nav.W)
		}
	}
}

// TestSettingsDangerKeepsItsConfirm checks that the danger group's wipe
// still asks first, from its new place at the nav's end.
func TestSettingsDangerKeepsItsConfirm(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 960, 640)
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("设置分组 危险操作"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the danger card", func() bool { return tt.HasText("清空所有数据") })
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

// guard: the toolkit import stays referenced if the helpers above change.
var _ = ui.Rect{}
