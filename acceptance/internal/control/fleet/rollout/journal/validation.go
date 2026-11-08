package journal

import (
	"fmt"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func ValidateRecord(value Record) error {
	if err := ValidateSpec(value.Spec); err != nil {
		return err
	}
	hash, err := safeio.Digest(value.Spec)
	if err != nil || value.Version != 1 || validate.Name(value.FleetID) != nil || hash != value.SpecSHA256 || value.Stage < 0 || value.Stage >= len(value.Spec.Stages) || len(value.Targets) != len(value.Spec.Stages) {
		return fmt.Errorf("invalid rollout identity or stage")
	}
	if value.CreatedAt <= 0 || value.UpdatedAt < value.CreatedAt || value.StageStartedAt < value.CreatedAt || value.StageStartedAt > value.UpdatedAt {
		return fmt.Errorf("invalid rollout timestamps")
	}
	if value.Phase != "running" && value.Phase != "succeeded" && value.Phase != "failed" && value.Phase != "canceled" {
		return fmt.Errorf("invalid rollout phase")
	}
	if value.Phase == "failed" && value.Failure == "" || value.Phase != "failed" && (value.Failure != "" || value.FailedNode != "") {
		return fmt.Errorf("invalid rollout failure record")
	}
	for index, stage := range value.Spec.Stages {
		if len(value.Targets[index]) != len(stage.Targets) {
			return fmt.Errorf("rollout target inventory mismatch")
		}
		for number, target := range stage.Targets {
			if err := validateTarget(value, target, value.Targets[index][number], index); err != nil {
				return err
			}
		}
	}
	if value.Phase == "succeeded" && value.Stage != len(value.Spec.Stages)-1 {
		return fmt.Errorf("rollout completed before the final stage")
	}
	return nil
}

func validateTarget(record Record, spec wire.RolloutTarget, value Target, stage int) error {
	if !validate.Hash(value.Generation) || value.RequestID != RequestID(record.SpecSHA256, spec.NodeID) || value.LastSeen < 0 || value.HealthySince < 0 || value.HealthySince > record.UpdatedAt {
		return fmt.Errorf("invalid rollout target identity or times")
	}
	if value.PublishedRevision == 0 {
		if value.Phase != "pending" || value.LastSeen != 0 || value.HealthySince != 0 || value.FirstReadyAttempt != 0 || stage < record.Stage || record.Phase == "succeeded" {
			return fmt.Errorf("invalid unpublished rollout target")
		}
		return nil
	}
	if stage > record.Stage || value.PublishedRevision <= spec.ExpectedRevision || value.Phase != "waiting" && value.Phase != "observing" && value.Phase != "ready" {
		return fmt.Errorf("invalid published rollout target")
	}
	if value.Phase == "waiting" && (value.HealthySince != 0 || value.FirstReadyAttempt != 0) || value.Phase != "waiting" && (value.HealthySince == 0 || value.FirstReadyAttempt == 0) {
		return fmt.Errorf("invalid rollout observation window")
	}
	if record.Phase == "succeeded" && value.Phase != "ready" {
		return fmt.Errorf("rollout completed without every target ready")
	}
	return nil
}
