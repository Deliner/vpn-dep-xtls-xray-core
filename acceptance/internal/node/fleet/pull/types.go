// Package pull owns durable enrollment and outbound Fleet reconciliation. The
// parent fleet package owns signature/replay checks and installer execution.
package pull

import (
	"context"
	"errors"
	"os"
	"sync"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/fleetapi"
)

var ErrClosed = errors.New("Fleet pull runner is closed")

type Bootstrap struct {
	Version    int             `json:"version"`
	Endpoint   string          `json:"endpoint"`
	CAPEM      []byte          `json:"ca_pem,omitempty"`
	Invitation wire.Invitation `json:"invitation"`
}

type connection struct {
	Version         int            `json:"version"`
	Endpoint        string         `json:"endpoint"`
	CAPEM           []byte         `json:"ca_pem,omitempty"`
	Trust           contract.Trust `json:"trust"`
	Credential      string         `json:"credential"`
	EnrollmentToken string         `json:"enrollment_token,omitempty"`
	Enrolled        bool           `json:"enrolled"`
}

type applier interface {
	Apply(context.Context, []byte) (wire.Status, error)
	Status() (wire.Status, error)
}

type Runner struct {
	mu        sync.Mutex
	directory string
	config    connection
	lock      *os.File
	client    *fleetapi.Client
	agent     applier
}
