package session

import (
	"context"
	"fmt"
	"os"
	"runtime"
)

// Resume takes over only a validated protected journal after its previous owner
// has exited. Recover never signals recorded PIDs. The caller owns a non-nil
// result even when launching fails, exactly as with Start.
func Resume(ctx context.Context, request Request, status Status) (*Session, error) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return nil, fmt.Errorf("resume requires Linux root")
	}
	if !resumable(status) {
		return nil, fmt.Errorf("resume requires an interrupted session with a recorded firewall lease")
	}
	if err := verifySnapshot(request); err != nil {
		return nil, err
	}
	if err := available(request); err != nil {
		return nil, err
	}
	s, err := Recover(ctx, request, status)
	if err != nil {
		return nil, err
	}
	// The old processes are gone; their recorded numeric IDs carry no authority.
	s.status.Phase, s.status.NativePID, s.status.FrontendPID = "failed-closed", 0, 0
	return s, s.Reconnect(ctx)
}

func resumable(status Status) bool {
	if status.Guard.Token == "" || status.Guard.Handle == 0 {
		return false
	}
	switch status.Phase {
	case "starting", "running", "reconnecting", "failed-closed":
		return true
	default:
		return false
	}
}

// Reconnect reuses only the trusted snapshot and the still-installed guard.
// The caller serializes it with Wait/Stop. It never releases or replaces nftables.
func (s *Session) Reconnect(ctx context.Context) error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 || s.lock == nil {
		return fmt.Errorf("reconnect requires the owned Linux root session")
	}
	if !s.protected || s.status.Guard.Token == "" || s.status.Phase != "failed-closed" {
		return fmt.Errorf("reconnect requires a failed-closed session with retained protection")
	}
	if err := s.prepareReconnect(ctx); err != nil {
		return err
	}
	s.group = newChildren()
	s.status = Status{Phase: "reconnecting", Guard: s.status.Guard}
	if err := s.save(); err != nil {
		return s.failed(err)
	}
	if err := s.launch(ctx); err != nil {
		return s.failed(err)
	}
	s.status.Phase = "running"
	if err := s.save(); err != nil {
		return s.failed(err)
	}
	return nil
}

func (s *Session) prepareReconnect(ctx context.Context) error {
	if err := s.guard.Verify(ctx, s.status.Guard); err != nil {
		return err
	}
	if err := s.linkGone(); err != nil {
		return err
	}
	if err := verifySnapshot(s.request); err != nil {
		return err
	}
	if err := available(s.request); err != nil {
		return err
	}
	// Removing stale owned selectors is safe while the original guard stays put.
	// Restore rejects foreign rules and requires the old TUN routes to be gone.
	return s.routes.Restore(ctx)
}
