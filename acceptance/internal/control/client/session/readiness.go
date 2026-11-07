package session

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/resolver"
)

func available(request Request) error {
	if err := network.Validate(request.Profile, request.Network); err != nil {
		return err
	}
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	for _, link := range interfaces {
		if link.Name == request.Network.Interface {
			return fmt.Errorf("client interface already exists; refusing to adopt it")
		}
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(request.Profile.Metadata.SOCKSPort)))
	if err != nil {
		return fmt.Errorf("client SOCKS port is already in use")
	}
	return listener.Close()
}

func (s *Session) waitSOCKS(ctx context.Context) error {
	return s.ready(ctx, func() bool {
		address := net.JoinHostPort("127.0.0.1", strconv.Itoa(s.request.Profile.Metadata.SOCKSPort))
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err != nil {
			return false
		}
		connection.Close()
		return true
	})
}

func (s *Session) waitTUN(ctx context.Context) error {
	return s.ready(ctx, func() bool {
		link, err := net.InterfaceByName(s.request.Network.Interface)
		if err != nil || link.Flags&net.FlagUp == 0 {
			return false
		}
		s.status.Link = resolver.Link{Name: link.Name, Index: link.Index}
		return true
	})
}

func (s *Session) ready(parent context.Context, check func() bool) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	tick := time.NewTicker(25 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("client readiness: %w", ctx.Err())
		case exit := <-s.group.exits:
			return fmt.Errorf("%s stopped during startup: %w", exit.name, exit.err)
		case <-tick.C:
			if check() {
				return nil
			}
		}
	}
}
