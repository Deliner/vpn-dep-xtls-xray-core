package server

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"time"
)

// Serve owns the listener until termination. The registry belongs to its caller
// and must remain open until all handlers have finished.
func (api *API) Serve(ctx context.Context, listener net.Listener, certificate tls.Certificate) error {
	if api.rollouts == nil {
		return api.serveHTTP(ctx, listener, certificate)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	worker, served := make(chan error, 1), make(chan error, 1)
	go func() { worker <- api.rollouts.Run(ctx) }()
	go func() { served <- api.serveHTTP(ctx, listener, certificate) }()
	select {
	case err := <-worker:
		cancel()
		return errors.Join(workerError(err), <-served)
	case err := <-served:
		cancel()
		return errors.Join(err, workerError(<-worker))
	}
}

func workerError(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func (api *API) serveHTTP(ctx context.Context, listener net.Listener, certificate tls.Certificate) error {
	server := &http.Server{Handler: api, ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: 60 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32 << 10,
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}}}
	done := make(chan error, 1)
	go func() { done <- server.ServeTLS(listener, "", "") }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := server.Shutdown(shutdown)
		if err != nil {
			err = errors.Join(err, server.Close())
		}
		served := <-done
		if !errors.Is(served, http.ErrServerClosed) {
			err = errors.Join(err, served)
		}
		return err
	}
}
