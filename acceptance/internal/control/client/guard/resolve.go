package guard

import (
	"context"
	"fmt"
)

// Resolve reads an identity from a trusted, locked journal. A pending intent
// accepts only its exact nonce and verified drop chains, or proven absence.
// A completed lease still requires its original kernel handle: a missing or
// replaced completed table is never reclassified as an unexecuted intention.
// Resolve never changes the firewall; callers must persist a completed lease
// before launching any engine or routing operation.
func (m *Manager) Resolve(ctx context.Context, lease Lease) (Lease, error) {
	if lease == (Lease{}) {
		return lease, m.Verify(ctx, lease)
	}
	if !tokenPattern.MatchString(lease.Token) {
		return Lease{}, fmt.Errorf("invalid recorded firewall identity")
	}
	if lease.Handle != 0 {
		return lease, m.Verify(ctx, lease)
	}
	actual, err := m.inspect(ctx)
	if err != nil {
		if err := m.Verify(ctx, Lease{}); err != nil {
			return Lease{}, fmt.Errorf("cannot resolve pending firewall intent: %w", err)
		}
		return Lease{}, nil
	}
	if actual.Token != lease.Token {
		return Lease{}, fmt.Errorf("pending firewall identity differs; protection retained")
	}
	return actual, m.verifyProtection(ctx)
}
