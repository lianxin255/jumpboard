package e2e

import (
	"testing"

	"github.com/lianxin255/jumpboard/internal/appt"
	"github.com/lianxin255/jumpboard/internal/model"
)

// TestTemplatePickerFillsTheForm opens the picker, picks MySQL, and checks
// the form came up with the template's defaults.
func TestTemplatePickerFillsTheForm(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	if err := tt.Click("从模板新建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("MySQL 数据库") {
		t.Fatalf("the templates should be listed: %v", tt.Texts())
	}

	if err := tt.Click("使用 MySQL 数据库"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	_, _, f, _ := a.FormState()
	if f.UpstreamPort != "3306" {
		t.Errorf("the MySQL template should prefill 3306: %+v", f)
	}
	if f.Protocol != appt.ProtocolLabel(model.TCP) {
		t.Errorf("the chosen template's protocol should be TCP: %q", f.Protocol)
	}
	if f.ExtraDirectives == "" {
		t.Error("the template's extra directives should carry over")
	}
	if !tt.HasText("新建映射") {
		t.Errorf("the form should be open: %v", tt.Texts())
	}
}

// TestCustomTemplateRoundTrip saves a template from the settings-less
// service, then finds it in the picker.
func TestCustomTemplateRoundTrip(t *testing.T) {
	a := testApp(t)
	before := len(a.Tpl.List())

	tpl := templateFrom("my-svc", "我的服务", "10.0.0.9", 8080)
	if err := a.Tpl.Save(tpl); err != nil {
		t.Fatal(err)
	}
	if got := len(a.Tpl.List()); got != before+1 {
		t.Fatalf("the template was not added: %d → %d", before, got)
	}

	tt := tester(t, a, 1200, 760)
	if err := tt.Click("从模板新建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("我的服务") {
		t.Errorf("the custom template should be listed: %v", tt.Texts())
	}

	if err := a.Tpl.Delete("my-svc"); err != nil {
		t.Fatal(err)
	}
	if got := len(a.Tpl.List()); got != before {
		t.Errorf("the template was not deleted: %d", got)
	}
}

// templateFrom builds a custom template for the tests.
func templateFrom(id, name, host string, port int) model.Template {
	return model.Template{
		ID: id, Name: name, Category: "Web", Description: "测试模板",
		Mapping: model.TemplateMapping{
			Protocol: model.TCP, UpstreamHost: host, UpstreamPort: port,
		},
		ExtraDirectives: "proxy_timeout 30m;",
	}
}
