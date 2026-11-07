package modules

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
)

func ResolveMissing(ctx context.Context, run commands.Runner, offset, limit int) (OriginBatch, error) {
	var inventory Report
	digest, err := records.Read("inventory.json", &inventory)
	if err != nil {
		return OriginBatch{}, err
	}
	if err := VerifyLocks(inventory); err != nil {
		return OriginBatch{}, err
	}
	requests := missingOrigins(inventory)
	if offset < 0 || offset >= len(requests) || limit < 1 || limit > 25 {
		return OriginBatch{}, fmt.Errorf("expected offset within %d missing origins and limit 1..25", len(requests))
	}
	if err := safeio.PrivateDir("build/dependencies/origin-cache"); err != nil {
		return OriginBatch{}, err
	}
	cache, err := filepath.Abs("build/dependencies/origin-cache")
	if err != nil {
		return OriginBatch{}, err
	}
	batch := OriginBatch{InventorySHA: digest, Offset: offset, Limit: limit, Results: []Resolution{}}
	for _, request := range requests[offset:min(offset+limit, len(requests))] {
		result, queryErr := queryOrigin(ctx, run, cache, request)
		batch.Results = append(batch.Results, result)
		if err := records.Save(fmt.Sprintf("origins-%04d.json", offset), batch); err != nil {
			return batch, err
		}
		if queryErr != nil {
			return batch, queryErr
		}
		if err := VerifyLocks(inventory); err != nil {
			return batch, err
		}
	}
	return batch, nil
}

func missingOrigins(report Report) []Module {
	unique := map[string]Module{}
	for _, scope := range report.Scopes {
		for _, module := range scope.Modules {
			if module.Main {
				continue
			}
			if module.Replace != nil {
				module = *module.Replace
			}
			if module.Origin == nil {
				unique[module.Path+"@"+module.Version] = module
			}
		}
	}
	var requests []Module
	for _, module := range unique {
		requests = append(requests, module)
	}
	sort.Slice(requests, func(i, j int) bool {
		return requests[i].Path+"@"+requests[i].Version < requests[j].Path+"@"+requests[j].Version
	})
	return requests
}
