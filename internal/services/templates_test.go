package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBuiltinReal(t *testing.T) {
	dir := "builtin"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[e.Name()] = b
	}
	got, err := LoadBuiltin(files)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	t.Logf("loaded %d templates", len(got))
	for id, tpl := range got {
		t.Logf("  %-14s %-10s proto=%-5s up=%d listen=%d", id, tpl.Category, tpl.Mapping.Protocol, tpl.Mapping.UpstreamPort, tpl.Mapping.ListenPort)
	}
}
