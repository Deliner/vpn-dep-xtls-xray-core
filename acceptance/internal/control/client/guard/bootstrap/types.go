// Package bootstrap retains an early-boot firewall until the session's durable
// kill switch is installed, or the user explicitly disconnects.
package bootstrap

import (
	"context"
	"os"

	"vpn-script/internal/control/client/guard"
	"vpn-script/internal/control/client/host"
)

const Directory = host.Directory + "/bootstrap"

type Status struct {
	Phase string      `json:"phase"`
	Guard guard.Lease `json:"guard"`
}

type journal struct {
	SchemaVersion int         `json:"schema_version"`
	Guard         guard.Lease `json:"guard"`
	Releasing     bool        `json:"releasing"`
}

type firewall interface {
	Verify(context.Context, guard.Lease) error
	Resolve(context.Context, guard.Lease) (guard.Lease, error)
	AcquireBootstrap(context.Context, guard.Lease) (guard.Lease, error)
	Release(context.Context, guard.Lease) error
}

type store struct {
	base     string
	uid, gid int
	lock     *os.File
	guard    firewall
}

func (s *store) close() {
	if s.lock != nil {
		s.lock.Close()
	}
}
