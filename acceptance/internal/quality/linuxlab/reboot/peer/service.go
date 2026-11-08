// Package peer runs controlled origins and a real VPN exit in the separate VM.
package peer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func Run(ctx context.Context, config contract.Config) error {
	s := &server{config: config, metrics: contract.PeerMetrics{DNSQueries: map[string]uint64{}, Canary: map[string]uint64{}, HTTPPeers: map[string]uint64{}, UDPPeers: map[string]uint64{}}}
	closers, err := s.listen(ctx)
	defer func() {
		for _, close := range closers {
			close()
		}
	}()
	if err != nil {
		return err
	}
	log, err := os.OpenFile(contract.Root+"/peer-engine.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	command := exec.CommandContext(ctx, contract.Root+"/bin/xray", "run", "-config", contract.Root+"/server.json")
	command.Stdout, command.Stderr = log, log
	command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
	if err := command.Start(); err != nil {
		return err
	}
	err = command.Wait()
	if ctx.Err() != nil {
		return nil
	}
	return fmt.Errorf("exit engine stopped unexpectedly: %w", err)
}

func (s *server) listen(ctx context.Context) ([]func(), error) {
	var closers []func()
	endpoints := []struct{ address, port string }{{s.config.PeerIP, "18081"}, {contract.OriginIP, "18081"}, {s.config.PeerIP, "18082"}, {s.config.PeerIP, "53"}}
	for _, endpoint := range endpoints {
		connection, err := net.ListenPacket("udp4", net.JoinHostPort(endpoint.address, endpoint.port))
		if err != nil {
			return closers, err
		}
		closers = append(closers, func() { connection.Close() })
		go s.serveUDP(connection, endpoint.port)
	}
	for _, port := range []string{"18080", "443"} {
		listener, err := s.httpListener(port)
		if err != nil {
			return closers, err
		}
		httpServer := &http.Server{Handler: http.HandlerFunc(s.serveHTTP), ReadHeaderTimeout: 2 * time.Second,
			BaseContext: func(net.Listener) context.Context { return ctx }}
		closers = append(closers, func() { httpServer.Close() })
		go httpServer.Serve(listener)
	}
	return closers, nil
}

func (s *server) httpListener(port string) (net.Listener, error) {
	address := net.JoinHostPort("0.0.0.0", port)
	if port != "443" {
		return net.Listen("tcp4", address)
	}
	certificate, err := tls.LoadX509KeyPair(contract.Root+"/dns.crt", contract.Root+"/dns.key")
	if err != nil {
		return nil, err
	}
	return tls.Listen("tcp4", address, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
}
