package fleet

import (
	"fmt"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/validate"
)

func (a *Agent) initialState() state {
	return state{Version: 1, FleetID: a.trust.FleetID, NodeID: a.trust.NodeID,
		TrustSHA256: contract.Digest(a.trust.PublicKey), Phase: "waiting"}
}

func (a *Agent) validateState(value state) error {
	identity := a.initialState()
	if value.Version != 1 || value.FleetID != identity.FleetID || value.NodeID != identity.NodeID || value.TrustSHA256 != identity.TrustSHA256 {
		return fmt.Errorf("Fleet state belongs to another identity or signing key")
	}
	if value.Phase == "waiting" && value.Accepted == (contract.Receipt{}) && value.Applied == (contract.Receipt{}) && value.Generation == "" && value.AttemptedAt == 0 && value.Attempt == 0 {
		return nil
	}
	if value.Accepted.Revision == 0 || !validate.Hash(value.Accepted.PayloadSHA256) || value.AttemptedAt <= 0 || value.Attempt == 0 {
		return fmt.Errorf("invalid accepted Fleet revision")
	}
	if err := validateApplied(value); err != nil {
		return err
	}
	switch value.Phase {
	case "applying", "failed":
		return nil
	case "ready":
		if value.Applied == value.Accepted {
			return nil
		}
	}
	return fmt.Errorf("invalid Fleet application phase")
}

func validateApplied(value state) error {
	if value.Applied == (contract.Receipt{}) && value.Generation == "" {
		return nil
	}
	if value.Applied.Revision == 0 || value.Applied.Revision > value.Accepted.Revision || !validate.Hash(value.Applied.PayloadSHA256) || !validate.Hash(value.Generation) {
		return fmt.Errorf("invalid applied Fleet revision")
	}
	if value.Applied.Revision == value.Accepted.Revision && value.Applied != value.Accepted {
		return fmt.Errorf("conflicting accepted and applied Fleet payloads")
	}
	return nil
}

func (value state) status() Status {
	return Status{FleetID: value.FleetID, NodeID: value.NodeID, Phase: value.Phase,
		AcceptedRevision: value.Accepted.Revision, AppliedRevision: value.Applied.Revision,
		Generation: value.Generation, AttemptedAt: value.AttemptedAt, Attempt: value.Attempt}
}
