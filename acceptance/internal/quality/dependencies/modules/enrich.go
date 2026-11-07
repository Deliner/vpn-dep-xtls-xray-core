package modules

import (
	"fmt"
	"path/filepath"
	"reflect"

	"vpn-script/internal/quality/dependencies/records"
)

func Enrich() (Report, error) {
	var report Report
	digest, err := records.Read("inventory.json", &report)
	if err != nil {
		return report, err
	}
	if err := VerifyLocks(report); err != nil {
		return report, err
	}
	missing := missingOrigins(report)
	resolved, err := resolvedOrigins(digest, missing)
	if err != nil {
		return report, err
	}
	if len(resolved) != len(missing) {
		return report, fmt.Errorf("origin resolution incomplete: %d of %d", len(resolved), len(missing))
	}
	for _, scope := range report.Scopes {
		_, targets := originRequests(scope.Modules)
		for key, modules := range targets {
			for _, module := range modules {
				if module.Origin == nil {
					module.Origin = resolved[key]
				}
			}
		}
	}
	if err := VerifyLocks(report); err != nil {
		return report, err
	}
	return report, records.Save("resolved-inventory.json", report)
}

func resolvedOrigins(digest string, missing []Module) (map[string]*Origin, error) {
	paths, err := filepath.Glob("build/dependencies/origins-*.json")
	if err != nil {
		return nil, err
	}
	resolved := map[string]*Origin{}
	for _, path := range paths {
		var batch OriginBatch
		if _, err := records.Read(filepath.Base(path), &batch); err != nil {
			return nil, err
		}
		if err := mergeBatch(resolved, batch, digest, missing); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	return resolved, nil
}

func mergeBatch(resolved map[string]*Origin, batch OriginBatch, digest string, missing []Module) error {
	if batch.InventorySHA != digest || batch.Offset < 0 || batch.Offset >= len(missing) || batch.Limit < 1 || batch.Limit > 25 || len(batch.Results) > batch.Limit || len(batch.Results) > len(missing)-batch.Offset {
		return fmt.Errorf("origin checkpoint does not belong to this inventory window")
	}
	for index, result := range batch.Results {
		request := missing[batch.Offset+index]
		if result.Path != request.Path || result.Version != request.Version {
			return fmt.Errorf("resolution does not match selected module/version at offset %d", batch.Offset+index)
		}
		if result.Failure != "" && result.Origin == nil {
			continue
		}
		if result.Failure != "" {
			return fmt.Errorf("failed origin resolution contains a source identity")
		}
		if err := checkOrigin(result.Origin); err != nil {
			return err
		}
		key := result.Path + "@" + result.Version
		if previous := resolved[key]; previous != nil && !reflect.DeepEqual(previous, result.Origin) {
			return fmt.Errorf("conflicting source identities for %s", key)
		}
		resolved[key] = result.Origin
	}
	return nil
}
