package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/client/guard"
	"vpn-script/internal/platform/safeio"
)

// Handoff verifies the committed session guard before removing boot protection.
// A failed/ambiguous cleanup retains it. A crash at deletion is replayable.
func Handoff(ctx context.Context, active guard.Lease) error {
	if active.Handle == 0 {
		return fmt.Errorf("bootstrap handoff requires a completed session firewall lease")
	}
	s, err := access(false)
	if err != nil {
		return err
	}
	defer s.close()
	m, err := guard.New()
	if err != nil {
		return err
	}
	if err := m.Verify(ctx, active); err != nil {
		return err
	}
	return s.release(ctx, false)
}

// Release is called only for an explicit stop, including before session start.
// It removes the released journal so a later explicit activation can rearm it.
func Release(ctx context.Context) error {
	s, err := access(false)
	if err != nil {
		return err
	}
	defer s.close()
	return s.release(ctx, true)
}

func (s *store) release(ctx context.Context, forget bool) error {
	value, found, err := s.read()
	if err != nil {
		return err
	}
	if !found {
		return s.guard.Verify(ctx, guard.Lease{})
	}
	lease, err := s.resolveRelease(ctx, value)
	if err != nil {
		return err
	}
	value.Releasing = true
	if lease.Handle != 0 {
		value.Guard = lease
	}
	if err := s.write(value); err != nil {
		return err
	}
	if lease.Handle != 0 {
		if err := s.guard.Release(ctx, lease); err != nil {
			return err
		}
	}
	if forget {
		if err := os.Remove(filepath.Join(s.base, "state.json")); err != nil {
			return err
		}
		return safeio.SyncDir(s.base)
	}
	return nil
}

func (s *store) resolveRelease(ctx context.Context, value journal) (guard.Lease, error) {
	lease, err := s.guard.Resolve(ctx, value.Guard)
	if err != nil && value.Releasing {
		// Only a persisted deletion intent can explain an absent completed lease.
		if absent := s.guard.Verify(ctx, guard.Lease{}); absent == nil {
			return guard.Lease{}, nil
		}
	}
	return lease, err
}
