package wire

import (
	"fmt"

	"vpn-script/internal/core/validate"
)

// Status omits manifests, credentials and raw installer errors. Attempt is a
// durable per-agent counter, independent of the desired-state revision.
type Status struct {
	FleetID          string `json:"fleet_id"`
	NodeID           string `json:"node_id"`
	Phase            string `json:"phase"`
	Attempt          uint64 `json:"attempt,string"`
	AcceptedRevision uint64 `json:"accepted_revision,string"`
	AppliedRevision  uint64 `json:"applied_revision,string"`
	Generation       string `json:"generation,omitempty"`
	AttemptedAt      int64  `json:"attempted_at"`
}

func (s Status) Validate(fleetID, nodeID string, desired uint64) error {
	if s.FleetID != fleetID || s.NodeID != nodeID || s.AppliedRevision > s.AcceptedRevision || s.AcceptedRevision > desired {
		return fmt.Errorf("status identity or revision mismatch")
	}
	if s.Phase == "waiting" && s.Attempt == 0 && s.AcceptedRevision == 0 && s.AppliedRevision == 0 && s.Generation == "" && s.AttemptedAt == 0 {
		return nil
	}
	if s.Attempt == 0 || s.AttemptedAt <= 0 || s.AcceptedRevision == 0 {
		return fmt.Errorf("invalid status attempt")
	}
	if s.AppliedRevision == 0 && s.Generation != "" || s.AppliedRevision > 0 && !validate.Hash(s.Generation) {
		return fmt.Errorf("invalid status generation")
	}
	switch s.Phase {
	case "applying", "failed":
		return nil
	case "ready":
		if s.AppliedRevision == s.AcceptedRevision {
			return nil
		}
	}
	return fmt.Errorf("invalid status phase")
}
