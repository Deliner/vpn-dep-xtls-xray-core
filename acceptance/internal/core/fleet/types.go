// Package fleet defines the signed desired-state contract shared by the Fleet
// controller and node agent. It performs no I/O and never installs a manifest.
package fleet

import (
	"crypto/ed25519"

	"vpn-script/internal/core/model"
)

const PayloadType = "application/vnd.nhop.fleet.assignment.v1+json"
const MaxManifestSize = model.MaxNodeConfigSize
const MaxEnvelopeSize = 24 << 20
const MaxValiditySeconds = 7 * 24 * 60 * 60
const ClockSkewSeconds = 120

type Assignment struct {
	Version        int    `json:"version"`
	FleetID        string `json:"fleet_id"`
	NodeID         string `json:"node_id"`
	Revision       uint64 `json:"revision,string"`
	IssuedAt       int64  `json:"issued_at"`
	ExpiresAt      int64  `json:"expires_at"`
	Manifest       []byte `json:"manifest"`
	ManifestSHA256 string `json:"manifest_sha256"`
}

// Receipt identifies the exact accepted payload. The agent must durably save it
// before installation; retries may use the same revision only with the same hash.
type Receipt struct {
	Revision      uint64 `json:"revision,string"`
	PayloadSHA256 string `json:"payload_sha256"`
}

// Trust is provisioned out of band. An envelope cannot introduce a trusted key.
type Trust struct {
	FleetID   string
	NodeID    string
	PublicKey ed25519.PublicKey
}
