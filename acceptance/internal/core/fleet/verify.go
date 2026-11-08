package fleet

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"time"

	"vpn-script/internal/core/validate"
)

// Accept authenticates one payload, validates its intended recipient and expiry,
// and rejects rollback or equivocation. It does not persist the returned receipt.
func Accept(raw []byte, trust Trust, previous Receipt, now time.Time) (Assignment, Receipt, error) {
	if err := trust.Validate(); err != nil {
		return Assignment{}, Receipt{}, err
	}
	payload, err := verifyPayload(raw, trust.PublicKey)
	if err != nil {
		return Assignment{}, Receipt{}, err
	}
	value, err := decodeAssignment(payload, now)
	if err != nil {
		return Assignment{}, Receipt{}, err
	}
	if value.FleetID != trust.FleetID || value.NodeID != trust.NodeID {
		return Assignment{}, Receipt{}, fmt.Errorf("assignment belongs to another fleet or node")
	}
	next := Receipt{Revision: value.Revision, PayloadSHA256: Digest(payload)}
	if err := verifyRevision(previous, next); err != nil {
		return Assignment{}, Receipt{}, err
	}
	return value, next, nil
}

func (trust Trust) Validate() error {
	if validate.Name(trust.FleetID) != nil || validate.Name(trust.NodeID) != nil || len(trust.PublicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid provisioned Fleet identity or signing key")
	}
	return nil
}

func decodeAssignment(payload []byte, now time.Time) (Assignment, error) {
	var value Assignment
	if err := json.Unmarshal(payload, &value); err != nil {
		return Assignment{}, fmt.Errorf("invalid assignment JSON")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(canonical, payload) {
		return Assignment{}, fmt.Errorf("assignment must use the v1 canonical field encoding")
	}
	if err := validateAssignment(value, now); err != nil {
		return Assignment{}, err
	}
	return value, nil
}

func verifyRevision(previous, next Receipt) error {
	if previous.Revision == 0 && previous.PayloadSHA256 == "" {
		return nil
	}
	if previous.Revision == 0 || !validate.Hash(previous.PayloadSHA256) {
		return fmt.Errorf("invalid persisted assignment receipt")
	}
	if next.Revision < previous.Revision || (next.Revision == previous.Revision && next.PayloadSHA256 != previous.PayloadSHA256) {
		return fmt.Errorf("assignment revision rollback or conflicting payload")
	}
	return nil
}
