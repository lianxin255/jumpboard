package appt

import (
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestFormGroupRoundTrip checks the group label survives the whole form
// cycle: mapping in, form filled, mapping out.
func TestFormGroupRoundTrip(t *testing.T) {
	m := model.Mapping{
		Name: "mysql", Group: "数据库", Protocol: model.TCP, Enabled: true,
		ListenPort: 9000, UpstreamHost: "127.0.0.1", UpstreamPort: 3306,
	}
	f := FormFromMapping(m)
	if f.Group != "数据库" {
		t.Fatalf("FormFromMapping group = %q, want %q", f.Group, "数据库")
	}
	got, errs, ok := f.Validate()
	if !ok {
		t.Fatalf("a valid mapping failed validation: %v", errs)
	}
	if got.Group != "数据库" {
		t.Fatalf("Validate group = %q, want %q", got.Group, "数据库")
	}

	// The blank side of the label: an ungrouped rule stays ungrouped
	// rather than inheriting a default.
	f.Group = ""
	got, _, ok = f.Validate()
	if !ok {
		t.Fatal("an empty group should validate")
	}
	if got.Group != "" {
		t.Fatalf("empty group became %q", got.Group)
	}
}

// TestFormGroupSanitized pins the two rules a typed group obeys: padded
// input is trimmed, and a label longer than fifty runes — the import
// path's cap — is cut to fifty rather than refused.
func TestFormGroupSanitized(t *testing.T) {
	f := NewForm()
	f.Name = "站"
	f.ListenPort = "9000"
	f.UpstreamHost = "127.0.0.1"
	f.UpstreamPort = "3306"

	f.Group = "  内网  "
	got, errs, ok := f.Validate()
	if !ok {
		t.Fatalf("validation: %v", errs)
	}
	if got.Group != "内网" {
		t.Fatalf("group not trimmed: %q", got.Group)
	}

	long := strings.Repeat("组", 60)
	f.Group = long
	got, errs, ok = f.Validate()
	if !ok {
		t.Fatalf("validation: %v", errs)
	}
	if want := strings.Repeat("组", 50); got.Group != want {
		t.Fatalf("group capped to %d runes, want 50", len([]rune(got.Group)))
	}
}

// TestGroupSuggestions checks what the form's autocomplete offers: every
// group in use, once each, empty groups skipped, in first-seen order.
func TestGroupSuggestions(t *testing.T) {
	a := droppedApp(t)
	for _, m := range []model.Mapping{
		{Name: "一", Group: "数据库", Protocol: model.TCP, Enabled: true,
			ListenPort: 19001, UpstreamHost: "127.0.0.1", UpstreamPort: 1},
		{Name: "二", Group: "内网", Protocol: model.TCP, Enabled: true,
			ListenPort: 19002, UpstreamHost: "127.0.0.1", UpstreamPort: 1},
		{Name: "三", Group: "数据库", Protocol: model.TCP, Enabled: true,
			ListenPort: 19003, UpstreamHost: "127.0.0.1", UpstreamPort: 1},
		{Name: "四", Protocol: model.TCP, Enabled: true,
			ListenPort: 19004, UpstreamHost: "127.0.0.1", UpstreamPort: 1},
	} {
		if _, err := a.Store.CreateMapping(m); err != nil {
			t.Fatal(err)
		}
	}
	got := a.GroupSuggestions()
	if len(got) != 2 || got[0] != "数据库" || got[1] != "内网" {
		t.Fatalf("GroupSuggestions = %v, want [数据库 内网]", got)
	}
}
