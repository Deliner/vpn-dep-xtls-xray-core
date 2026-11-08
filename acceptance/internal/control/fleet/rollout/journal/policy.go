package journal

import (
	"fmt"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/safeio"
)

func ValidateSpec(value wire.RolloutSpec) error {
	if err := value.Validate(); err != nil {
		return fmt.Errorf("%w: %v", registry.ErrInvalid, err)
	}
	return nil
}

func RequestID(specSHA, nodeID string) string {
	return safeio.Hash([]byte("nhop-rollout/v1\n" + specSHA + "\n" + nodeID))
}
