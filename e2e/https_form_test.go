package e2e

import (
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/lianxin255/nginx-jump-gui/internal/model"
)

// chooseProtocol switches the form's protocol dropdown, the way a user
// does: click the dropdown, click the option.
func chooseProtocol(t *testing.T, tt *ui.Tester, want string) {
	t.Helper()
	if err := tt.Click("TCP"); err != nil {
		t.Fatalf("opening the protocol dropdown: %v", err)
	}
	tt.Frame()
	if err := tt.Click(want); err != nil {
		t.Fatalf("choosing %q: %v", want, err)
	}
	tt.Frame()
}

// TestHTTPSFormSavesAllFields fills the HTTPS form through the window,
// including the fields that only exist once the protocol says HTTPS, and
// checks that every one of them reaches the database.
func TestHTTPSFormSavesAllFields(t *testing.T) {
	a := testApp(t)
	tt := tester(t, a, 1200, 760)

	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	writeTestCert(t, certPath, keyPath)
	listen := freePort(t)

	if err := tt.Click("新建映射"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	chooseProtocol(t, tt, "HTTPS")

	// The HTTPS-only fields are on the form now.
	if !tt.HasText("TLS 证书") || !tt.HasText("HTTP 跳转 HTTPS") {
		t.Fatalf("the HTTPS fields did not appear: %v", tt.Texts())
	}

	// An HTTPS rule without a certificate is refused before it is saved.
	fillField(t, tt, "名称", "安全站")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("HTTPS 需要证书和私钥路径") || !tt.HasText("HTTPS 需要 server_name") {
		t.Errorf("the certificate errors should show: %v", tt.Texts())
	}
	if list, _ := a.Store.ListMappings(); len(list) != 0 {
		t.Errorf("an HTTPS rule without a certificate was saved: %+v", list)
	}

	fillField(t, tt, "server_name", "secure.example.com")
	fillField(t, tt, "监听端口", itoa(listen))
	fillField(t, tt, "上游主机", "127.0.0.1")
	fillField(t, tt, "上游端口", "8443")

	// The certificate fields sit below the fold; the user scrolls, and so
	// does the test.
	tt.Scroll(600, 380, 0, -600)
	tt.Frame()
	fillField(t, tt, "TLS 证书", certPath)
	fillField(t, tt, "TLS 私钥", keyPath)
	if err := tt.Click("HTTP 跳转 HTTPS"); err != nil {
		t.Fatalf("toggling the redirect: %v", err)
	}
	tt.Frame()

	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the mapping in the table", func() bool {
		list, _ := a.Store.ListMappings()
		return len(list) == 1
	})

	list, err := a.Store.ListMappings()
	if err != nil {
		t.Fatal(err)
	}
	m := list[0]
	if m.Name != "安全站" || m.Protocol != model.HTTPS ||
		m.ServerNames != "secure.example.com" || m.ListenPort != listen ||
		m.TLSCertPath != certPath || m.TLSKeyPath != keyPath || !m.RedirectHTTP {
		t.Errorf("stored wrongly: %+v", m)
	}
}
