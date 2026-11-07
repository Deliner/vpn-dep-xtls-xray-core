package session

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"time"

	"vpn-script/internal/control/client/guard"
	"vpn-script/internal/platform/safeio"
)

// Wait observes engine failure or controller cancellation. Neither authorizes
// opening the direct network: both stop children and retain the kill switch.
func (s *Session) Wait(ctx context.Context) error {
	if s.status.Phase != "running" {
		return fmt.Errorf("client is not running")
	}
	select {
	case <-ctx.Done():
		return s.failed(ctx.Err())
	case exit := <-s.group.exits:
		return s.failed(fmt.Errorf("%s stopped unexpectedly: %v", exit.name, exit.err))
	}
}

// Stop is the explicit request to restore ordinary networking. A failed cleanup
// retains protection and can be retried. It never removes another table/link.
func (s *Session) Stop(ctx context.Context) error {
	if s.status.Phase == "stopped" {
		return nil
	}
	err := s.restoreDNS(ctx)
	s.group.stop()
	if err = errors.Join(err, s.linkGone()); err == nil {
		err = s.routes.Restore(ctx)
	}
	if err == nil && s.status.Guard.Token != "" {
		err = s.guard.Release(ctx, s.status.Guard)
	}
	if err != nil {
		s.status.Phase = "failed-closed"
		return errors.Join(err, s.save())
	}
	s.status.Phase, s.status.Guard = "stopped", guard.Lease{}
	s.protected = false
	s.status.NativePID, s.status.FrontendPID = 0, 0
	return errors.Join(s.save(), s.unlock())
}

// Abort ends the controller's ownership without removing firewall protection.
// Recovery requires inspecting the persisted lease; Start never adopts it.
func (s *Session) Abort() error {
	if s.status.Phase == "stopped" {
		return s.unlock()
	}
	return errors.Join(s.failed(fmt.Errorf("controller aborted; protection retained")), s.unlock())
}

func (s *Session) failed(cause error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.restoreDNS(ctx)
	s.group.stop()
	s.status.Phase = "failed-closed"
	if !s.protected {
		s.status.Phase = "failed"
	}
	s.status.NativePID, s.status.FrontendPID = 0, 0
	return errors.Join(cause, err, s.save())
}

func (s *Session) restoreDNS(ctx context.Context) error {
	if s.status.Link.Index == 0 || s.linkGone() == nil {
		return nil
	}
	return s.dns.Revert(ctx, s.status.Link)
}

func (s *Session) linkGone() error {
	links, err := net.Interfaces()
	if err != nil {
		return err
	}
	for _, link := range links {
		if link.Name == s.request.Network.Interface || link.Index == s.status.Link.Index {
			return fmt.Errorf("owned TUN still exists or was replaced; protection retained")
		}
	}
	return nil
}

func (s *Session) save() error {
	return safeio.JSON(filepath.Join(s.request.Directory, "status.json"), s.status)
}

func (s *Session) unlock() error {
	if s.lock == nil {
		return nil
	}
	err := s.lock.Close()
	s.lock = nil
	return err
}
