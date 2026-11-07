package catalog

import (
	"fmt"
	"reflect"
	"strings"

	"vpn-script/internal/quality/dependencies/modules"
	"vpn-script/internal/quality/dependencies/records"
)

// BuildAdditional preserves the first plan's offsets and schedules only origins
// discovered later. Explicit source/commit mappings preserve external provenance.
func BuildAdditional() (Plan, error) {
	plan, err := additionalPlan()
	if err != nil {
		return plan, err
	}
	return plan, records.Save("additional-plan.json", plan)
}

func additionalPlan() (Plan, error) {
	initial, baseSHA, err := loadPlan()
	if err != nil {
		return Plan{}, err
	}
	var inventory modules.Report
	digest, err := records.Read("resolved-inventory.json", &inventory)
	if err != nil {
		return Plan{}, err
	}
	if err := modules.VerifyLocks(inventory); err != nil {
		return Plan{}, err
	}
	mirrors, mirrorsSHA, err := readMirrors()
	if err != nil {
		return Plan{}, err
	}
	plan := applyMirrors(makePlan(inventory), mirrors)
	plan.InventorySHA, plan.BasePlanSHA, plan.MirrorsSHA = digest, baseSHA, mirrorsSHA
	return excludeInitial(plan, initial), nil
}

func excludeInitial(plan, initial Plan) Plan {
	seen := map[string]bool{}
	for _, entry := range initial.Entries {
		seen[strings.ToLower(entry.Upstream)] = true
	}
	remaining := []Entry{}
	for _, entry := range plan.Entries {
		if !seen[strings.ToLower(entry.Upstream)] {
			remaining = append(remaining, entry)
		}
	}
	plan.Entries = remaining
	return plan
}

func loadAdditionalPlan() (Plan, string, error) {
	var plan Plan
	digest, err := records.Read("additional-plan.json", &plan)
	if err != nil {
		return plan, "", err
	}
	expected, err := additionalPlan()
	if err != nil {
		return plan, "", err
	}
	if !reflect.DeepEqual(plan, expected) {
		return plan, "", fmt.Errorf("additional plan does not match current inventory, base plan or mirror mappings")
	}
	return plan, digest, nil
}
