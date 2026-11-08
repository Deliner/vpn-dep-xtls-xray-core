package wire

import (
	"fmt"
	"math"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/validate"
)

const MaxRolloutNodes = 10000

func (value RolloutSpec) Validate() error {
	if value.Version != 1 || validate.Name(value.ID) != nil || len(value.Stages) < 2 || len(value.Stages) > MaxRolloutNodes {
		return fmt.Errorf("rollout requires an identifier, canary and later stages")
	}
	if value.ObserveSeconds < 1 || value.ObserveSeconds > 86400 || value.FreshSeconds < 1 || value.FreshSeconds > 3600 || value.StageTimeoutSeconds <= value.ObserveSeconds || value.StageTimeoutSeconds > contract.MaxValiditySeconds || value.ValiditySeconds < 1 || value.ValiditySeconds > contract.MaxValiditySeconds {
		return fmt.Errorf("rollout observation, freshness, deadline or validity")
	}
	names, nodes := map[string]bool{}, map[string]bool{}
	for _, stage := range value.Stages {
		if validate.Name(stage.Name) != nil || names[stage.Name] || len(stage.Targets) == 0 || len(stage.Targets) > 1000 {
			return fmt.Errorf("stage name or size")
		}
		names[stage.Name] = true
		for _, target := range stage.Targets {
			if validate.Name(target.NodeID) != nil || nodes[target.NodeID] || !validate.Hash(target.CandidateSHA256) || target.ExpectedRevision == math.MaxUint64 {
				return fmt.Errorf("duplicate or invalid rollout target")
			}
			nodes[target.NodeID] = true
		}
	}
	if len(nodes) > MaxRolloutNodes {
		return fmt.Errorf("too many rollout targets")
	}
	return nil
}
