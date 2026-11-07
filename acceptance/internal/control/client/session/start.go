package session

import (
	"context"
	"fmt"
	"os"
	"runtime"

	"vpn-script/internal/control/client/guard"
	"vpn-script/internal/control/client/resolver"
	"vpn-script/internal/control/client/routing"
)

// Start returns a non-nil session after taking ownership, including on failure.
// The caller must retain it to request an explicit Stop or fail-closed Abort.
// EngineUID/GID must belong exclusively to this client's non-login account.
func Start(ctx context.Context, request Request) (*Session, error) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return nil, fmt.Errorf("client activation requires Linux root; no host settings changed")
	}
	if err := available(request); err != nil {
		return nil, err
	}
	routes, err := routing.New(ctx, request.Profile.Metadata.EntryEndpoint.Address, request.Network.Interface, request.Network.IPv6 == "tunnel")
	if err != nil {
		return nil, err
	}
	lock, err := prepare(request)
	if err != nil {
		return nil, err
	}
	s := &Session{request: request, lock: lock, routes: routes, status: Status{Phase: "starting"}, group: &children{exits: make(chan childExit, 2)}}
	s.guard, _ = guard.New()
	s.dns, _ = resolver.New()
	if err := s.activate(ctx); err != nil {
		return s, s.failed(err)
	}
	s.status.Phase = "running"
	if err := s.save(); err != nil {
		return s, s.failed(err)
	}
	return s, nil
}

func (s *Session) activate(ctx context.Context) error {
	var err error
	s.status.Guard, err = s.guard.Acquire(ctx, s.request.Profile, s.request.Network)
	if err != nil {
		return err
	}
	s.protected = true
	if err := s.save(); err != nil {
		return err
	}
	s.status.NativePID, err = s.group.start(s.request, "native", s.request.Profile.Metadata.Engine, true)
	if err != nil {
		return err
	}
	if err := s.waitSOCKS(ctx); err != nil {
		return err
	}
	s.status.FrontendPID, err = s.group.start(s.request, "frontend", "sing-box", false)
	if err != nil {
		return err
	}
	if err := s.waitTUN(ctx); err != nil {
		return err
	}
	if err := s.routes.Apply(ctx, s.status.Link); err != nil {
		return err
	}
	return s.dns.ConfigureNewLink(ctx, s.status.Link)
}
