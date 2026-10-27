package e2e

import (
	"fmt"
	"testing"
)

func TestProbeConfirmPaste(t *testing.T) {
	a := testApp(t)
	if _, err := a.Store.CreateMapping(streamRule(t, "已有规则")); err != nil {
		t.Fatal(err)
	}
	if err := a.Reload(); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	a.ReadClipboard = func() (string, error) {
		return fmt.Sprintf("server {\n  listen %d;\n  proxy_pass http://192.168.1.20:8080;\n}\n", port), nil
	}
	if err := a.StartPasteImport(); err != nil {
		t.Fatal(err)
	}
	if err := a.ConfirmPasteImport(); err != nil {
		t.Fatalf("ConfirmPasteImport: %v", err)
	}
	list, _ := a.Store.ListMappings()
	t.Logf("mappings = %d", len(list))
	for _, m := range list {
		t.Logf("  %+v", m)
	}
}
