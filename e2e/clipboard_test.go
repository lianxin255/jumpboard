package e2e

import (
	"strings"
	"testing"

	"github.com/lianxin255/jumpboard/internal/model"
)

// TestCopyAddress checks that clicking a rule's port puts its public
// address on the clipboard.
func TestCopyAddress(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(model.Mapping{
		Name: "复制", Protocol: model.TCP, Enabled: true, ListenPort: 9000,
		UpstreamHost: "10.0.0.1", UpstreamPort: 3306,
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	tt := tester(t, a, 1200, 760)
	if err := tt.Click("复制地址-复制"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, tt, "the address copied", func() bool {
		return strings.Contains(a.Copied(), "9000")
	})
	if got := a.Copied(); got != "本机:9000" {
		t.Errorf("copied %q", got)
	}
}
