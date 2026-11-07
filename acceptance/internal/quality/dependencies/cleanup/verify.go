package cleanup

import (
	"context"
	"fmt"

	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
)

type Verification struct {
	PlanSHA string `json:"plan_sha256"`
	Kept    int    `json:"kept"`
	Removed int    `json:"removed"`
}

func Verify(ctx context.Context, run commands.Runner, digest string) (Verification, error) {
	result := Verification{PlanSHA: digest}
	plan, err := loadPlan(digest)
	if err != nil {
		return result, err
	}
	if err := authenticate(ctx, run); err != nil {
		return result, err
	}
	live, _, err := repositories(ctx, run)
	if err != nil {
		return result, err
	}
	if err := verifyInventory(plan, live); err != nil {
		return result, err
	}
	result.Kept, result.Removed = len(plan.Keep), len(plan.Delete)
	return result, records.Save("cleanup-verification.json", result)
}

func verifyInventory(plan Plan, live []Repository) error {
	byID, byName := map[int64]Repository{}, map[string]bool{}
	for _, repo := range live {
		byID[repo.ID], byName[repo.Name] = repo, true
	}
	for _, kept := range plan.Keep {
		repo, found := byID[kept.ID]
		if !found || repo.Name != kept.Name || repo.Owner.ID != accountID || repo.Owner.Login != "Deliner" {
			return fmt.Errorf("protected repository missing or renamed: %s", kept.Name)
		}
	}
	for _, target := range plan.Delete {
		if _, found := byID[target.ID]; found || byName[target.Name] {
			return fmt.Errorf("cleanup target still exists: %s", target.Name)
		}
	}
	return nil
}
