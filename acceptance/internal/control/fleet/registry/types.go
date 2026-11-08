package registry

import (
	"crypto/ed25519"
	"errors"
	"os"
	"sync"
	"time"

	"vpn-script/internal/core/fleet/wire"
)

var ErrUnauthorized = errors.New("Fleet credential rejected")
var ErrRevoked = errors.New("Fleet access revoked")
var ErrNotFound = errors.New("Fleet node not found")
var ErrConflict = errors.New("Fleet state changed or operation conflicts")
var ErrInvalid = errors.New("invalid Fleet request")
var ErrClosed = errors.New("Fleet registry is closed")

type Store struct {
	mu        sync.Mutex
	directory string
	authority authority
	key       ed25519.PrivateKey
	lock      *os.File
	now       func() time.Time
}

type authority struct {
	Version   int    `json:"version"`
	FleetID   string `json:"fleet_id"`
	Seed      []byte `json:"seed"`
	AdminHash string `json:"admin_hash"`
}

type adminCredential struct {
	Token string `json:"token"`
}

type nodeRecord struct {
	Version          int          `json:"version"`
	FleetID          string       `json:"fleet_id"`
	NodeID           string       `json:"node_id"`
	Group            string       `json:"group"`
	EnrollmentHash   string       `json:"enrollment_hash"`
	EnrollmentEnd    int64        `json:"enrollment_end"`
	CredentialHash   string       `json:"credential_hash,omitempty"`
	EnrolledAt       int64        `json:"enrolled_at"`
	RevokedAt        int64        `json:"revoked_at"`
	Desired          wire.Desired `json:"desired"`
	Status           *wire.Status `json:"status,omitempty"`
	LastSeen         int64        `json:"last_seen"`
	StatusReceivedAt int64        `json:"status_received_at,omitempty"`
}
