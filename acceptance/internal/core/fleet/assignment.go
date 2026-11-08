package fleet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"vpn-script/internal/core/validate"
)

func NewAssignment(fleetID, nodeID string, revision uint64, manifest []byte, now time.Time, validity time.Duration) (Assignment, error) {
	if validity < time.Second || validity > MaxValiditySeconds*time.Second || validity%time.Second != 0 {
		return Assignment{}, fmt.Errorf("assignment validity must be whole seconds within 1..%d", MaxValiditySeconds)
	}
	value := Assignment{Version: 1, FleetID: fleetID, NodeID: nodeID, Revision: revision,
		IssuedAt: now.Unix(), ExpiresAt: now.Add(validity).Unix(), Manifest: bytes.Clone(manifest), ManifestSHA256: Digest(manifest)}
	if err := validateAssignment(value, now); err != nil {
		return Assignment{}, err
	}
	return value, nil
}

func validateAssignment(value Assignment, now time.Time) error {
	if value.Version != 1 || validate.Name(value.FleetID) != nil || validate.Name(value.NodeID) != nil || value.Revision == 0 {
		return fmt.Errorf("invalid assignment schema or identity")
	}
	if len(value.Manifest) == 0 || len(value.Manifest) > MaxManifestSize || !json.Valid(value.Manifest) {
		return fmt.Errorf("invalid assignment manifest encoding or size")
	}
	if !validate.Hash(value.ManifestSHA256) || Digest(value.Manifest) != value.ManifestSHA256 {
		return fmt.Errorf("assignment manifest checksum mismatch")
	}
	return validateValidity(value, now)
}

func validateValidity(value Assignment, now time.Time) error {
	current := now.Unix()
	if current <= 0 || value.IssuedAt <= 0 || value.ExpiresAt <= value.IssuedAt {
		return fmt.Errorf("invalid assignment timestamps")
	}
	if value.ExpiresAt-value.IssuedAt > MaxValiditySeconds {
		return fmt.Errorf("assignment validity exceeds policy")
	}
	if value.IssuedAt > current && value.IssuedAt-current > ClockSkewSeconds {
		return fmt.Errorf("assignment was issued in the future")
	}
	if current >= value.ExpiresAt {
		return fmt.Errorf("assignment has expired")
	}
	return nil
}
