package catalog

import (
	"sort"
	"strings"

	"vpn-script/internal/quality/dependencies/modules"
	"vpn-script/internal/quality/dependencies/records"
)

const inventoryPath = "build/dependencies/inventory.json"
const planPath = "build/dependencies/plan.json"

func Build() (Plan, error) {
	return buildPlan("inventory.json", "plan.json")
}

func BuildResolved() (Plan, error) {
	return buildPlan("resolved-inventory.json", "resolved-plan.json")
}

func buildPlan(inventoryName, planName string) (Plan, error) {
	var report modules.Report
	digest, err := records.Read(inventoryName, &report)
	if err != nil {
		return Plan{}, err
	}
	if err := modules.VerifyLocks(report); err != nil {
		return Plan{}, err
	}
	plan := makePlan(report)
	plan.InventorySHA = digest
	return plan, records.Save(planName, plan)
}

func makePlan(report modules.Report) Plan {
	plan := Plan{Schema: 1, Entries: []Entry{}, Unresolved: []Unresolved{}}
	entries := map[string]*Entry{}
	for _, scope := range report.Scopes {
		for _, module := range scope.Modules {
			if module.Main {
				continue
			}
			if module.Replace != nil {
				module = *module.Replace
			}
			name, reason := source(module)
			ref := reference(scope.Name, module)
			if reason != "" {
				item := Unresolved{Reference: ref, Reason: reason}
				if module.Origin != nil {
					item.URL = module.Origin.URL
				}
				plan.Unresolved = append(plan.Unresolved, item)
				continue
			}
			key := strings.ToLower(name)
			if entries[key] == nil {
				entries[key] = &Entry{Upstream: name}
			}
			entries[key].References = append(entries[key].References, ref)
		}
	}
	for _, entry := range entries {
		plan.Entries = append(plan.Entries, *entry)
	}
	sort.Slice(plan.Entries, func(i, j int) bool {
		return strings.ToLower(plan.Entries[i].Upstream) < strings.ToLower(plan.Entries[j].Upstream)
	})
	return plan
}
