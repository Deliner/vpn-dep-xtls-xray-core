// Package fleet applies authenticated desired state through the existing node
// installer. Network polling and controller authentication are separate concerns.
package fleet

import (
	"context"
	"time"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
)

type Agent struct {
	directory string
	trust     contract.Trust
	install   func(context.Context, model.NodeConfig) (installer.Result, error)
	now       func() time.Time
}

type Status = wire.Status

type state struct {
	Version     int              `json:"version"`
	FleetID     string           `json:"fleet_id"`
	NodeID      string           `json:"node_id"`
	TrustSHA256 string           `json:"trust_sha256"`
	Accepted    contract.Receipt `json:"accepted"`
	Applied     contract.Receipt `json:"applied"`
	Generation  string           `json:"generation,omitempty"`
	Phase       string           `json:"phase"`
	Attempt     uint64           `json:"attempt,string"`
	AttemptedAt int64            `json:"attempted_at"`
}
