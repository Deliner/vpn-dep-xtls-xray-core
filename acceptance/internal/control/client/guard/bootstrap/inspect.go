package bootstrap

import (
	"context"

	"vpn-script/internal/control/client/guard"
)

func Inspect(ctx context.Context) (Status, error) {
	s, err := access(false)
	if err != nil {
		return Status{}, err
	}
	defer s.close()
	return s.inspect(ctx)
}

func (s *store) inspect(ctx context.Context) (Status, error) {
	value, found, err := s.read()
	if err != nil {
		return Status{}, err
	}
	if !found {
		return Status{Phase: "absent"}, s.guard.Verify(ctx, guard.Lease{})
	}
	lease, err := s.resolveRelease(ctx, value)
	if err != nil {
		return Status{}, err
	}
	phase := "protected"
	if lease == (guard.Lease{}) {
		phase = "pending"
		if value.Releasing {
			phase = "released"
		}
	}
	return Status{Phase: phase, Guard: lease}, nil
}
