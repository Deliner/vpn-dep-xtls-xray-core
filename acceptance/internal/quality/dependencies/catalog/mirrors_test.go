package catalog

import (
	"strings"
	"testing"

	"vpn-script/internal/quality/dependencies/modules"
)

func TestMirrorMappingRequiresTheExactSourceAndCommit(t *testing.T) {
	dep := module("native", "https://source.example/lib")
	mapping := mirror{Source: dep.Origin.URL, Repository: "mirror/repo", Commit: dep.Origin.Hash}
	mirrors := map[string]mirror{mapping.Source + "\x00" + mapping.Commit: mapping}
	other := module("different-revision", dep.Origin.URL)
	other.Origin.Hash = strings.Repeat("b", 40)
	plan := makePlan(modules.Report{Scopes: []modules.Scope{{Name: "scope", Modules: []modules.Module{dep, other, module("github", "https://github.com/Mirror/Repo")}}}})
	got := applyMirrors(plan, mirrors)
	if len(got.Entries) != 1 || len(got.Entries[0].References) != 2 || len(got.Unresolved) != 1 || got.Unresolved[0].Commit != other.Origin.Hash {
		t.Fatalf("crossed source identity or lost remaining origin: %+v", got)
	}
	ref := got.Entries[0].References[1]
	if ref.SourceURL != mapping.Source || ref.Commit != mapping.Commit {
		t.Fatalf("lost original provenance: %+v", ref)
	}
}

func TestMirrorTableRejectsAmbiguousIdentities(t *testing.T) {
	valid := []string{"https://source.example/lib", "mirror/repo", strings.Repeat("a", 40), strings.Repeat("b", 40)}
	for index, replacement := range []string{"https://user@source.example/lib", "../wrong", "short", "short"} {
		row := append([]string{}, valid...)
		row[index] = replacement
		if _, err := parseMirrors([][]string{row}); err == nil {
			t.Fatalf("accepted invalid column %d", index)
		}
	}
	if _, err := parseMirrors([][]string{valid, valid}); err == nil {
		t.Fatal("accepted duplicate source/commit")
	}
	if _, err := parseMirrors([][]string{valid[:3]}); err == nil {
		t.Fatal("accepted missing tree identity")
	}
}
