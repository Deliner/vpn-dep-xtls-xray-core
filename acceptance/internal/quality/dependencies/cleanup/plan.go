package cleanup

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
	"vpn-script/internal/quality/dependencies/scope"
)

func Build(ctx context.Context, run commands.Runner) (Plan, error) {
	if err := authenticate(ctx, run); err != nil {
		return Plan{}, err
	}
	keep, digest, err := scope.Read()
	if err != nil {
		return Plan{}, err
	}
	recorded, err := recordedTargets()
	if err != nil {
		return Plan{}, err
	}
	live, raw, err := repositories(ctx, run)
	if err != nil {
		return Plan{}, err
	}
	plan, err := classify(live, recorded, keep)
	if err != nil {
		return plan, err
	}
	plan.ScopeSHA, plan.InventorySHA = digest, safeio.Hash(raw)
	return plan, records.Save("cleanup-plan.json", plan)
}

func classify(live []Repository, recorded map[int64]Target, keep []scope.Fork) (Plan, error) {
	plan := Plan{Schema: 1, AccountID: accountID, Keep: keep, Delete: []Target{}, Unknown: []string{}, Absent: []string{}}
	present := map[int64]bool{}
	for _, repo := range live {
		present[repo.ID] = true
		target, found := recorded[repo.ID]
		if !found {
			if strings.HasPrefix(repo.Name, "Deliner/vpn-dep-") {
				plan.Unknown = append(plan.Unknown, repo.Name)
			}
			continue
		}
		if protected(target, keep) {
			continue
		}
		if err := verifyIdentity(repo, target, keep); err != nil {
			return plan, err
		}
		target.CreatedAt = repo.CreatedAt
		plan.Delete = append(plan.Delete, target)
	}
	for id, target := range recorded {
		if !present[id] {
			plan.Absent = append(plan.Absent, target.Name)
		}
	}
	for _, fork := range keep {
		if !present[fork.ID] {
			return plan, fmt.Errorf("maintained repository missing from live inventory: %s", fork.Name)
		}
	}
	sort.Slice(plan.Delete, func(i, j int) bool { return plan.Delete[i].Name < plan.Delete[j].Name })
	return plan, nil
}
