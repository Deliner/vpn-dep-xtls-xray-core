package diagnostics

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryRuleHasReadableRepairSkill(t *testing.T) {
	for _, rule := range rules {
		var output bytes.Buffer
		if err := Explain("../../..", rule, &output); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		if !strings.Contains(text, "refactor/SKILL.md") || !strings.Contains(text, string(rule)+"/SKILL.md") {
			t.Fatalf("missing repair routing for %s", rule)
		}
	}
}

func TestEveryDiagnosticNamesItsRepairSkill(t *testing.T) {
	for _, rule := range rules {
		message := Issue(rule, "sample.go:17: violation").String()
		for _, expected := range []string{"sample.go:17: violation", "Для исправления используй skill `" + string(rule) + "`",
			rule.SkillPath(), "just explain " + string(rule)} {
			if !strings.Contains(message, expected) {
				t.Fatalf("%s has no explicit repair instruction: %s", rule, message)
			}
		}
	}
}

func TestReportPrintsSelectedGuideOnlyOnceAndFailsIfMissing(t *testing.T) {
	var output bytes.Buffer
	items := []Finding{Issue(DirectorySize, "one"), Issue(DirectorySize, "two")}
	if err := Report("../../..", items, &output); err != nil {
		t.Fatal(err)
	}
	if strings.Count(output.String(), "Read and apply ops/quality/skills/directory-size/SKILL.md") != 1 {
		t.Fatal("guide not deduplicated")
	}
	if strings.Count(output.String(), DirectorySize.Instruction()) != len(items) {
		t.Fatal("each error must name its own repair skill before the shared guide")
	}
	root := t.TempDir()
	path := filepath.Join(root, "ops/quality/skills/refactor")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "SKILL.md"), []byte("workflow"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Report(root, items, &output); err == nil {
		t.Fatal("missing repair skill was ignored")
	}
}
