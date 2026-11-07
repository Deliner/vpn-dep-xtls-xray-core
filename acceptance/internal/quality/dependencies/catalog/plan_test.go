package catalog

import (
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/modules"
)

func TestPlanGroupsRepositoriesAndPreservesUnresolvedSources(t *testing.T) {
	first := module("a", "https://github.com/Owner/Repo")
	second := module("b", "https://github.com/owner/repo.git")
	second.Version = "v2.0.0"
	third := modules.Module{Path: "replaced", Version: "v1", Replace: &second}
	missing := modules.Module{Path: "missing", Version: "v1"}
	external := module("external", "https://go.googlesource.com/net")
	report := modules.Report{Scopes: []modules.Scope{{Name: "one", Modules: []modules.Module{first, third, missing, external}}}}
	plan := makePlan(report)
	if len(plan.Entries) != 1 || len(plan.Entries[0].References) != 2 || len(plan.Unresolved) != 2 {
		t.Fatalf("incomplete repository plan: %+v", plan)
	}
	if plan.Entries[0].References[1].Module != "b" || plan.Entries[0].References[1].Version != "v2.0.0" {
		t.Fatal("lost selected replacement identity")
	}
}

func TestPlanRejectsNoncanonicalSourceURLs(t *testing.T) {
	for _, address := range []string{"http://github.com/o/r", "https://github.com.evil/o/r", "https://user@github.com/o/r", "https://github.com/o/r?x=1", "https://github.com/o/r#x", "https://github.com/o/r/more", "https://github.com/o/%72", "https://github.com/../r"} {
		if _, reason := source(module("test", address)); reason == "" {
			t.Fatalf("accepted ambiguous source %s", address)
		}
	}
	partial := module("test", "https://github.com/o/r")
	partial.Origin.Hash = "abc"
	if _, reason := source(partial); reason == "" {
		t.Fatal("accepted abbreviated commit")
	}
}

func TestLoadPlanRejectsAlteredDestination(t *testing.T) {
	fixture(t)
	plan, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	plan.Entries[0].Upstream = "different/repository"
	if err := safeio.JSON(planPath, plan); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadPlan(); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("altered fork destination accepted: %v", err)
	}
}

func TestLoadPlanRejectsChangedDependencyLocks(t *testing.T) {
	fixture(t)
	if _, err := Build(); err != nil {
		t.Fatal(err)
	}
	write(t, "go.sum", "changed")
	if _, _, err := loadPlan(); err == nil || !strings.Contains(err.Error(), "stale checksum") {
		t.Fatalf("stale fork plan accepted: %v", err)
	}
}
