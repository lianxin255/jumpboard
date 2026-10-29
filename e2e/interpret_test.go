package e2e

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"

	"github.com/lianxin255/jumpboard/internal/model"
	"github.com/lianxin255/jumpboard/internal/nginx"
)

// TestAIInterpretDroppedConf walks the import's follow-up end to end: the
// drop merges, the bar offers the interpretation, the card explains the
// conf — and the rule set is untouched, field for field. An
// interpretation writes nothing back, ever.
func TestAIInterpretDroppedConf(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(a.ActiveWorkspace(), model.Mapping{
		Name: "已有", Protocol: model.TCP, Enabled: true, ListenPort: 16001,
		UpstreamHost: "10.9.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	conf := nginx.Generate([]model.Mapping{
		{Name: "拖入的库", Protocol: model.TCP, Enabled: true, ListenPort: 16002,
			UpstreamHost: "10.9.0.2", UpstreamPort: 6379},
	}, nginx.GenOptions{})
	path := filepath.Join(t.TempDir(), "dropped.conf")
	if err := os.WriteFile(path, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := fakeOpenAI(t, `{"summary":"这份配置把一个内网库转发到本机",`+
		`"listeners":["127.0.0.1:16002"],"forwards":["10.9.0.2:6379"],`+
		`"risks":["上游是明文转发"],"notes":["确认 16002 没有对公网开放"]}`)
	a.SetAI(srv.URL, "test-model")

	dropAndAwait(t, a, 2, path)
	before, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(before) != 2 {
		t.Fatalf("the drop should leave two rules: %v %v", before, err)
	}

	tt := tester(t, a, 1200, 760)
	waitFor(t, tt, "the import bar", func() bool { return tt.HasText("已导入 dropped.conf") })
	if err := tt.Click("AI 解读"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the interpretation card", func() bool {
		return tt.HasText("这份配置把一个内网库转发到本机")
	})
	for _, want := range []string{
		"谁在听", "转发关系", "127.0.0.1:16002", "10.9.0.2:6379", "注意事项", "仅供参考",
	} {
		if !tt.HasText(want) {
			t.Errorf("the card should show %q: %v", want, tt.Texts())
		}
	}

	// The interpretation is display only: the rule set is exactly what the
	// drop saved, field for field.
	after, err := a.Store.ListMappings(a.ActiveWorkspace())
	if err != nil || len(after) != 2 {
		t.Fatalf("the interpretation must not touch the rules: %v %v", after, err)
	}
	byName := map[string]model.Mapping{}
	for _, m := range after {
		byName[m.Name] = m
	}
	for _, want := range before {
		got := byName[want.Name]
		if got.ListenPort != want.ListenPort || got.UpstreamHost != want.UpstreamHost ||
			got.UpstreamPort != want.UpstreamPort || got.Enabled != want.Enabled ||
			got.Protocol != want.Protocol {
			t.Errorf("%s changed under the interpretation: %+v -> %+v", want.Name, want, got)
		}
	}
}

// TestAIInterpretFromFilePicker walks the logs page's picker path: a
// file chosen in the dialog — here an external nginx's error.log — is
// read, interpreted, and explained on the card under the logs page's
// toolbar, the same card the 访问日志 interpretation lands on.
func TestAIInterpretFromFilePicker(t *testing.T) {
	a := testApp(t)
	errLog := "2026/10/26 10:00:00 [error] 8#0: *1 connect() failed (111: Connection refused) " +
		"while connecting to upstream, upstream: \"10.9.0.9:6379\"\n" +
		"2026/10/26 10:00:01 [error] 8#0: *2 no live upstreams while connecting to upstream, " +
		"upstream: \"10.9.0.9:6379\"\n"
	path := filepath.Join(t.TempDir(), "error.log")
	if err := os.WriteFile(path, []byte(errLog), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := fakeOpenAI(t, `{"summary":"上游 502","listeners":[],"forwards":[],`+
		`"risks":["上游连不上"],"notes":["看 upstream"]}`)
	a.SetAI(srv.URL, "test-model")
	// The dialog hook is per-app; main.go points it at the native dialog,
	// a test hands over the path directly.
	a.OpenDialog = func(mygo.OpenDialogOptions) ([]string, error) {
		return []string{path}, nil
	}

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("访问日志"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	// The picker's button needs only AI: it reads an outside file, not
	// the engine's own log.
	if err := tt.Click("解读文件…"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the interpretation", func() bool { return tt.HasText("上游 502") })
	for _, want := range []string{"error.log", "冲突与风险", "仅供参考"} {
		if !tt.HasText(want) {
			t.Errorf("the card should show %q: %v", want, tt.Texts())
		}
	}
	// Each testApp is its own instance, so the hook cannot leak; it is
	// cleared anyway so this file reads as self-contained.
	a.OpenDialog = nil
}

// TestAIInterpretFilePickerErrorIsVisible separates an OS dialog failure
// from cancellation: the error reaches the app's toast with its cause.
func TestAIInterpretFilePickerErrorIsVisible(t *testing.T) {
	a := testApp(t)
	srv := fakeOpenAI(t, `{"summary":"不应请求","listeners":[],"forwards":[],"risks":[],"notes":[]}`)
	a.SetAI(srv.URL, "test-model")
	a.OpenDialog = func(mygo.OpenDialogOptions) ([]string, error) {
		return nil, errors.New("native dialog unavailable")
	}
	tt := tester(t, a, 1200, 760)
	if err := tt.Click("日志"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("访问日志"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click("解读文件…"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("打开文件选择器失败") || !tt.HasText("native dialog unavailable") {
		t.Fatalf("the dialog error should be visible with context: %v", tt.Texts())
	}
	st, ok := a.Status()
	if !ok || !st.IsErr {
		t.Fatalf("the dialog failure should be an error status: %+v ok=%v", st, ok)
	}
}
