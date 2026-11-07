package documents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/quality/diagnostics"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLocalLinksAndFencedExamples(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "nested/a file.md"), "target\n")
	path := filepath.Join(root, "README.md")
	write(t, path, "[ok](<nested/a file.md>)\n[web](https://example.com/missing)\n```md\n[example](fake.md)\n```\n[bad](missing.md)\n")
	findings, err := Links(path)
	if err != nil || len(findings) != 1 || !strings.Contains(findings[0], "missing.md") {
		t.Fatalf("link results: %v, %v", findings, err)
	}
}

func TestInventoryDetectsChangedDocument(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, Inventory), "old inventory\n")
	path := filepath.Join(root, Research, "topic/note.md")
	write(t, path, "original\n")
	if err := WriteInventory(root); err != nil {
		t.Fatal(err)
	}
	if err := CheckInventory(root); err != nil {
		t.Fatal(err)
	}
	write(t, path, "changed\n")
	if err := CheckInventory(root); err == nil {
		t.Fatal("changed content did not invalidate inventory")
	}
}

func TestDocumentChecksSelectDistinctRepairSkills(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, Inventory), "stale inventory\n")
	write(t, filepath.Join(root, "README.md"), "[missing](missing.md)\n")
	findings, err := Check(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[diagnostics.Rule]bool{}
	for _, finding := range findings {
		seen[finding.Rule] = true
	}
	if len(findings) != 2 || !seen[diagnostics.DocumentationLinks] || !seen[diagnostics.ResearchInventory] {
		t.Fatalf("document checks did not select their own skills: %v", findings)
	}
}
