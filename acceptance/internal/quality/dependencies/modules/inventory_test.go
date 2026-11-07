package modules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInventoryPreservesActualReplacementSource(t *testing.T) {
	raw := `{"Path":"main","Main":true}
{"Path":"example.org/module","Version":"v1.0.0","Replace":{"Path":"github.com/fork/module","Version":"v1.0.1","Origin":{"VCS":"git","URL":"https://github.com/fork/module","Hash":"abc","Subdir":"nested"}}}`
	modules, err := decodeModules([]byte(raw))
	if err != nil || len(modules) != 2 {
		t.Fatalf("%+v, %v", modules, err)
	}
	replacement := modules[1].Replace
	if replacement == nil || replacement.Origin == nil || replacement.Origin.URL != "https://github.com/fork/module" || replacement.Origin.Subdir != "nested" {
		t.Fatalf("lost selected source: %+v", replacement)
	}
}

func TestInventoryRejectsIncompleteGraphs(t *testing.T) {
	main := `{"Path":"main","Main":true}`
	for _, raw := range []string{"", `{`, `{"Path":"dep","Version":"v1"}`,
		main + `{"Path":"dep"}`, main + `{"Path":"dep","Error":{"Err":"missing"}}`,
		main + main, main + `{"Path":"dep","Version":"v1","Replace":{"Path":"../local"}}`} {
		if _, err := decodeModules([]byte(raw)); err == nil {
			t.Fatalf("accepted incomplete graph: %s", raw)
		}
	}
}

func TestInventoryRefusesLocksChangedDuringCollection(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("before"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		if err := os.WriteFile(filepath.Join(root, "go.sum"), []byte("after"), 0600); err != nil {
			t.Fatal(err)
		}
		return []byte(`{"Path":"main","Main":true}`), nil
	}
	_, err := collectScope(context.Background(), run, Scope{Directory: root})
	if err == nil || !strings.Contains(err.Error(), "locks changed") {
		t.Fatalf("changed checksum file accepted: %v", err)
	}
}
