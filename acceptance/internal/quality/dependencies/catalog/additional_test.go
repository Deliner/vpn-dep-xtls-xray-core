package catalog

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestAdditionalPlanPreservesInitialOffsetsAndProvenance(t *testing.T) {
	fixture(t)
	if _, err := Build(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	resolvedFixture(t)
	plan, err := BuildAdditional()
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(planPath)
	if err != nil || string(before) != string(after) {
		t.Fatal("additional planning changed initial offsets")
	}
	if len(plan.Entries) != 2 || len(plan.Unresolved) != 1 || plan.BasePlanSHA == "" || plan.MirrorsSHA == "" {
		t.Fatalf("incomplete additional plan: %+v", plan)
	}
	if plan.Entries[0].Upstream != "mirror/repo" || plan.Entries[1].Upstream != "new/repo" || plan.Entries[0].References[0].SourceURL != "https://source.example/lib" {
		t.Fatalf("lost new origin or mirror evidence: %+v", plan)
	}
	if _, _, err := loadAdditionalPlan(); err != nil {
		t.Fatal(err)
	}
}

func TestAdditionalBatchRefusesChangedMirrorBeforeGitHub(t *testing.T) {
	fixture(t)
	if _, err := Build(); err != nil {
		t.Fatal(err)
	}
	resolvedFixture(t)
	if _, err := BuildAdditional(); err != nil {
		t.Fatal(err)
	}
	write(t, "ops/dependencies/mirrors.tsv", "https://source.example/lib\tdifferent/repo\t"+strings.Repeat("a", 40)+"\t"+strings.Repeat("b", 40)+"\n")
	run := func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("changed additional plan contacted GitHub")
		return nil, nil
	}
	if _, err := ApplyAdditional(context.Background(), run, 0, 1, "Deliner"); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("accepted changed mapping: %v", err)
	}
}
