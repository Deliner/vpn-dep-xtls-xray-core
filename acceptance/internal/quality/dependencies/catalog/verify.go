package catalog

import (
	"fmt"
	"reflect"

	"vpn-script/internal/quality/dependencies/modules"
	"vpn-script/internal/quality/dependencies/records"
)

func loadPlan() (Plan, string, error) {
	var plan Plan
	planDigest, err := records.Read("plan.json", &plan)
	if err != nil {
		return plan, "", err
	}
	var inventory modules.Report
	digest, err := records.Read("inventory.json", &inventory)
	if err != nil {
		return plan, "", err
	}
	if err := modules.VerifyLocks(inventory); err != nil {
		return plan, "", err
	}
	expected := makePlan(inventory)
	expected.InventorySHA = digest
	if !reflect.DeepEqual(plan, expected) {
		return plan, "", fmt.Errorf("plan does not match current module inventory; regenerate it")
	}
	return plan, planDigest, nil
}
