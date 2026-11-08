// Package wire contains the Fleet API's data contracts. Authentication,
// persistence and installation belong to their respective adapters.
package wire

type Enrollment struct {
	NodeID     string `json:"node_id"`
	Token      string `json:"token"`
	Credential string `json:"credential"`
}

type Identity struct {
	FleetID string `json:"fleet_id"`
	NodeID  string `json:"node_id"`
}

type Invite struct {
	NodeID          string `json:"node_id"`
	Group           string `json:"group"`
	ValiditySeconds int64  `json:"validity_seconds"`
}

type Invitation struct {
	FleetID   string `json:"fleet_id"`
	NodeID    string `json:"node_id"`
	PublicKey []byte `json:"public_key"`
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type Publish struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision uint64 `json:"expected_revision,string"`
	ValiditySeconds  int64  `json:"validity_seconds"`
	Manifest         []byte `json:"manifest"`
}

type Desired struct {
	Revision       uint64 `json:"revision,string"`
	Generation     string `json:"generation"`
	EnvelopeSHA256 string `json:"envelope_sha256"`
	PayloadSHA256  string `json:"payload_sha256"`
	ManifestSHA256 string `json:"manifest_sha256"`
	RequestID      string `json:"request_id"`
	PublicationID  string `json:"publication_id,omitempty"`
	IssuedAt       int64  `json:"issued_at"`
	ExpiresAt      int64  `json:"expires_at"`
}

// OriginRequestID survives lease renewal. For older records without this field,
// their last stored request becomes the origin of subsequent renewals.
func (d Desired) OriginRequestID() string {
	if d.PublicationID != "" {
		return d.PublicationID
	}
	return d.RequestID
}

type Node struct {
	FleetID          string  `json:"fleet_id"`
	NodeID           string  `json:"node_id"`
	Group            string  `json:"group"`
	Enrolled         bool    `json:"enrolled"`
	Revoked          bool    `json:"revoked"`
	Desired          Desired `json:"desired"`
	Status           *Status `json:"status,omitempty"`
	LastSeen         int64   `json:"last_seen"`
	StatusReceivedAt int64   `json:"status_received_at,omitempty"`
}
