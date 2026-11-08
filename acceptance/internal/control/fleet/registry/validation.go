package registry

import (
	"fmt"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/validate"
)

func validateRecord(value nodeRecord) error {
	if value.Version != 1 || validate.Name(value.FleetID) != nil || validate.Name(value.NodeID) != nil || validate.Name(value.Group) != nil {
		return fmt.Errorf("invalid Fleet node record identity")
	}
	if !validate.Hash(value.EnrollmentHash) || value.EnrollmentEnd <= 0 || value.EnrolledAt < 0 || value.RevokedAt < 0 || value.LastSeen < 0 {
		return fmt.Errorf("invalid Fleet enrollment record")
	}
	if value.EnrolledAt == 0 && value.CredentialHash != "" || value.EnrolledAt > 0 && !validate.Hash(value.CredentialHash) {
		return fmt.Errorf("invalid Fleet credential record")
	}
	if err := validateDesired(value.Desired); err != nil {
		return err
	}
	if value.StatusReceivedAt < 0 || value.StatusReceivedAt > value.LastSeen || value.Status == nil && value.StatusReceivedAt != 0 {
		return fmt.Errorf("invalid status receipt time")
	}
	if value.Status != nil {
		if value.EnrolledAt == 0 || value.LastSeen == 0 {
			return fmt.Errorf("status without enrollment or receive time")
		}
		return value.Status.Validate(value.FleetID, value.NodeID, value.Desired.Revision)
	}
	return nil
}

func validateDesired(value wire.Desired) error {
	if value == (wire.Desired{}) {
		return nil
	}
	if value.Revision == 0 || value.IssuedAt <= 0 || value.ExpiresAt <= value.IssuedAt || value.ExpiresAt-value.IssuedAt > contract.MaxValiditySeconds {
		return fmt.Errorf("invalid stored assignment revision or validity")
	}
	for _, hash := range []string{value.Generation, value.EnvelopeSHA256, value.PayloadSHA256, value.ManifestSHA256, value.RequestID} {
		if !validate.Hash(hash) {
			return fmt.Errorf("invalid stored assignment digest")
		}
	}
	if value.PublicationID != "" && !validate.Hash(value.PublicationID) {
		return fmt.Errorf("invalid publication origin")
	}
	return nil
}

func validateInvite(value wire.Invite) error {
	if validate.Name(value.NodeID) != nil || validate.Name(value.Group) != nil || value.ValiditySeconds < 1 || value.ValiditySeconds > 24*60*60 {
		return fmt.Errorf("%w: enrollment identity or lifetime", ErrInvalid)
	}
	return nil
}
