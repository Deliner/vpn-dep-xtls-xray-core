package app

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"path/filepath"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout"
	"vpn-script/internal/control/fleet/server"
	"vpn-script/internal/platform/safeio"
)

func serve(ctx context.Context, o options) error {
	if err := protectedParent(o.state); err != nil {
		return err
	}
	certificate, err := loadCertificate(o.cert, o.key)
	if err != nil {
		return err
	}
	store, err := registry.Open(o.state)
	if err != nil {
		return err
	}
	defer store.Close()
	coordinator, err := rollout.Open(filepath.Join(o.state, "rollouts"), store)
	if err != nil {
		return err
	}
	defer coordinator.Close()
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", o.listen)
	if err != nil {
		return err
	}
	return server.New(store, coordinator).Serve(ctx, listener, certificate)
}

func loadCertificate(certPath, keyPath string) (tls.Certificate, error) {
	if err := protectedParent(keyPath); err != nil {
		return tls.Certificate{}, err
	}
	if err := safeio.CheckAccess(keyPath, 0600, os.Geteuid(), -1); err != nil {
		return tls.Certificate{}, err
	}
	key, err := safeio.Read(keyPath, 1<<20)
	if err != nil {
		return tls.Certificate{}, err
	}
	cert, err := safeio.Read(certPath, 1<<20)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(cert, key)
}
