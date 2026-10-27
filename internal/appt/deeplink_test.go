package appt

import (
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/paths"
)

// newTestApp builds an app over a throwaway data dir, the way every other
// appt test does.
func newTestApp(t *testing.T) *App {
	t.Helper()
	t.Setenv("JUMPBOARD_DATA_ROOT", t.TempDir())
	t.Setenv("JUMPBOARD_NO_KEYCHAIN", "1")
	p, err := paths.Resolve()
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	return a
}

// TestParseDeeplink pins the three answers a jumpboard:// URL gets: show,
// import, refusal — and the refusals a URL deserves.
func TestParseDeeplink(t *testing.T) {
	for _, tc := range []struct {
		name  string
		raw   string
		show  bool
		pay   string
		errIn string
	}{
		{name: "bare scheme shows the window", raw: "jumpboard://", show: true},
		{name: "open shows the window", raw: "jumpboard://open", show: true},
		{name: "import carries the data", raw: "jumpboard://import?data=server%20%7B%7D", pay: "server {}"},
		{name: "import without data is refused", raw: "jumpboard://import", errIn: "缺少内容"},
		{name: "a foreign scheme is refused", raw: "https://example.com", errIn: "不是 JumpBoard 链接"},
		{name: "an unknown action is refused", raw: "jumpboard://wipe", errIn: "不认识"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dl := ParseDeeplink(tc.raw)
			if tc.errIn != "" {
				if dl.Err == "" || !strings.Contains(dl.Err, tc.errIn) {
					t.Fatalf("err = %q, want it to contain %q", dl.Err, tc.errIn)
				}
				return
			}
			if dl.Err != "" {
				t.Fatalf("unexpected refusal: %s", dl.Err)
			}
			if dl.Show != tc.show {
				t.Errorf("show = %v, want %v", dl.Show, tc.show)
			}
			if dl.Payload != tc.pay {
				t.Errorf("payload = %q, want %q", dl.Payload, tc.pay)
			}
		})
	}
}

// TestParseDeeplinkRefusesHugePayload: past 4 KB a link is told to become
// a paste instead.
func TestParseDeeplinkRefusesHugePayload(t *testing.T) {
	huge := "jumpboard://import?data=" + url.QueryEscape(strings.Repeat("a", 5<<10))
	dl := ParseDeeplink(huge)
	if dl.Err == "" || !strings.Contains(dl.Err, "粘贴导入") {
		t.Fatalf("err = %q, want the paste advice", dl.Err)
	}
}

// TestHandleDeeplinkImportRunsThePreview: an import URL lands on the same
// preview a paste does, and confirming it imports the content.
func TestHandleDeeplinkImportRunsThePreview(t *testing.T) {
	a := newTestApp(t)
	conf := "server {\n  listen 19000;\n  proxy_pass http://10.9.8.7:1234;\n}\n"
	raw := "jumpboard://import?data=" + url.QueryEscape(conf)

	if err := a.HandleDeeplink(raw); err != nil {
		t.Fatal(err)
	}
	pv := a.PastePreview()
	if pv == nil {
		t.Fatal("the import URL should raise the preview")
	}
	if !strings.Contains(pv.Detail, "1 条映射") {
		t.Errorf("preview detail: %s", pv.Detail)
	}
	if err := a.ConfirmPasteImport(); err != nil {
		t.Fatal(err)
	}
	list, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(list) != 1 || list[0].UpstreamHost != "10.9.8.7" {
		t.Fatalf("imported: %v %v", list, err)
	}
}

// TestHandleDeeplinkShowNeedsNoWindow: a show URL without the window hook
// is a quiet no-op, which is the state a deeplink arrives in before the
// dock exists.
func TestHandleDeeplinkShowNeedsNoWindow(t *testing.T) {
	a := newTestApp(t)
	shown := false
	a.ShowWindow = func() { shown = true }
	if err := a.HandleDeeplink("jumpboard://open"); err != nil {
		t.Fatal(err)
	}
	if !shown {
		t.Error("a show URL should call the hook")
	}
	a.ShowWindow = nil
	if err := a.HandleDeeplink("jumpboard://open"); err != nil {
		t.Fatalf("a nil hook should not fail: %v", err)
	}
}

// TestHandleDeeplinkRefuses is the error path the toast shows.
func TestHandleDeeplinkRefuses(t *testing.T) {
	a := newTestApp(t)
	err := a.HandleDeeplink(fmt.Sprintf("jumpboard://import?data=%s", url.QueryEscape("随便一段话")))
	if err == nil || !strings.Contains(err.Error(), "无法识别") {
		t.Fatalf("err = %v, want the unrecognized refusal", err)
	}
}
