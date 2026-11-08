package bootstrap

import (
	"context"
	"fmt"

	"vpn-script/internal/control/client/guard"
	"vpn-script/internal/control/client/host"
)

// Activate serializes with the foreground controller. It needs no account,
// profile, DNS or route, and saves the identity before touching nftables.
func Activate(ctx context.Context) error {
	owner, err := host.Acquire()
	if err != nil {
		return err
	}
	defer owner.Close()
	s, err := access(true)
	if err != nil {
		return err
	}
	defer s.close()
	return s.activate(ctx)
}

func (s *store) activate(ctx context.Context) error {
	value, found, err := s.read()
	if err != nil {
		return err
	}
	if found && value.Releasing {
		return fmt.Errorf("bootstrap already handed off; explicit stop required before rearming")
	}
	if !found {
		if err := s.guard.Verify(ctx, guard.Lease{}); err != nil {
			return err
		}
		value = journal{SchemaVersion: 1, Guard: guard.NewIntent()}
		if err := s.write(value); err != nil {
			return err
		}
	}
	lease, err := s.guard.Resolve(ctx, value.Guard)
	if err != nil {
		return err
	}
	if lease == (guard.Lease{}) {
		lease, err = s.guard.AcquireBootstrap(ctx, value.Guard)
		if err != nil {
			return err
		}
	}
	value.Guard = lease
	return s.write(value)
}
