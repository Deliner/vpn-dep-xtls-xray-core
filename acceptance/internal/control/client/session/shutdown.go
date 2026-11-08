package session

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"vpn-script/internal/control/client/guard"
	"vpn-script/internal/control/client/guard/bootstrap"
	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/resolver"
	"vpn-script/internal/control/client/routing"
	"vpn-script/internal/platform/safeio"
)

// Recover accepts only a trusted root-owned journal after the controller lock
// has been acquired. It never signals recorded PIDs or adopts a surviving TUN.
// The caller must explicitly Stop to release the verified persistent guard.
func Recover(ctx context.Context, request Request, status Status) (*Session, error) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return nil, fmt.Errorf("client recovery requires Linux root")
	}
	if err := network.Validate(request.Profile, request.Network); err != nil {
		return nil, err
	}
	s := &Session{request: request, status: status, group: newChildren()}
	if err := s.linkGone(); err != nil {
		return nil, err
	}
	s.guard, _ = guard.New()
	s.dns, _ = resolver.New()
	if err := s.recoverProtection(ctx); err != nil {
		return nil, err
	}
	path := filepath.Join(request.Directory, "session.lock")
	if err := safeio.CheckAccess(path, 0600, 0, 0); err != nil {
		return nil, err
	}
	var err error
	s.lock, err = safeio.Lock(path)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Session) recoverProtection(ctx context.Context) error {
	lease, err := s.guard.Resolve(ctx, s.status.Guard)
	if err != nil {
		return err
	}
	routes, err := recoveryRoutes(ctx, s.request, s.status.Guard)
	if err != nil {
		return err
	}
	s.routes, s.status.Guard = routes, lease
	s.protected = lease != (guard.Lease{})
	return nil
}

func recoveryRoutes(ctx context.Context, request Request, lease guard.Lease) (*routing.Manager, error) {
	entry, iface, ipv6 := request.Profile.Metadata.EntryEndpoint.Address, request.Network.Interface, request.Network.IPv6 == "tunnel"
	if lease.Handle == 0 {
		// Routes cannot have been installed before acquiring the guard. Without a
		// completed lease, require vacancy instead of adopting matching selectors.
		return routing.New(ctx, entry, iface, ipv6)
	}
	return routing.Recorded(entry, iface, ipv6)
}

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
		return bootstrap.Release(ctx)
	}
	if s.status.Guard.Handle == 0 {
		if err := s.recoverProtection(ctx); err != nil {
			return err
		}
	}
	err := s.restoreDNS(ctx)
	s.group.stop()
	if err = errors.Join(err, s.linkGone()); err == nil {
		err = s.routes.Restore(ctx)
	}
	if err == nil {
		err = bootstrap.Release(ctx)
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
