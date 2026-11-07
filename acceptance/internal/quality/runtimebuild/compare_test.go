package runtimebuild

import (
	"path/filepath"
	"testing"
)

func TestComparisonRequiresBothUnchangedBuilds(t *testing.T) {
	first, second := auditFixture(t, "xray"), auditFixture(t, "xray")
	firstPath := filepath.Join(first.root, outputDirectory)
	secondPath := filepath.Join(second.root, outputDirectory)
	if output, err := first.command("compare.sh", firstPath, secondPath).CombinedOutput(); err != nil {
		t.Fatalf("identical independent builds rejected: %v, %s", err, output)
	}
	write(t, filepath.Join(secondPath, "source/added.go"), []byte("package unexpected\n"), 0600)
	if output, err := first.command("compare.sh", firstPath, secondPath).CombinedOutput(); err == nil {
		t.Fatalf("changed input ignored during comparison: %s", output)
	}
}
