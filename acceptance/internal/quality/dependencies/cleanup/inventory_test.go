package cleanup

import (
	"strings"
	"testing"

	"vpn-script/internal/quality/dependencies/scope"
)

func TestRepositoryPagesKeepEveryPageAndRejectTruncatedInventory(t *testing.T) {
	rows, err := decodePages([]byte("[{\"id\":1}]\n[{\"id\":2}]\n"))
	if err != nil || len(rows) != 2 || rows[0].ID != 1 || rows[1].ID != 2 {
		t.Fatalf("incomplete paginated inventory: %v %v", rows, err)
	}
	if _, err := decodePages([]byte("[{\"id\":1}]\n[{")); err == nil {
		t.Fatal("partial inventory accepted")
	}
}

func TestCleanupVerificationRequiresEveryProtectedIDAndEveryRemoval(t *testing.T) {
	f := newFixture(t)
	plan := Plan{Keep: []scope.Fork{{ID: f.repo.ID, Name: f.repo.Name}}, Delete: []Target{{ID: 99, Name: "Deliner/vpn-dep-excess"}}}
	if err := verifyInventory(plan, []Repository{f.repo}); err != nil {
		t.Fatal(err)
	}
	if err := verifyInventory(plan, nil); err == nil || !strings.Contains(err.Error(), "protected") {
		t.Fatalf("lost protected repository was missed: %v", err)
	}
	extra := f.repo
	extra.ID, extra.Name = 99, plan.Delete[0].Name
	if err := verifyInventory(plan, []Repository{f.repo, extra}); err == nil {
		t.Fatal("remaining excess repository was missed")
	}
}
