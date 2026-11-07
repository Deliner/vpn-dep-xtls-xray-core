package integration

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"vpn-script/internal/control/integration/evidence"
)

var payload = bytes.Repeat([]byte("0123456789abcdef"), 2048)

func matrixAddresses(t *testing.T, hops int, addresses []string) []string {
	t.Helper()
	if len(addresses) == 0 {
		addresses = []string{"127.0.0.1"}
	}
	if len(addresses) != 1 && len(addresses) != hops {
		t.Fatal("node addresses must name every hop or one repeated loopback")
	}
	result := make([]string, hops)
	for hop := range hops {
		address := addresses[hop%len(addresses)]
		if address != "127.0.0.1" && address != "::1" {
			t.Fatal("payload node addresses must remain on loopback")
		}
		result[hop] = address
	}
	return result
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func rejectingProxy(t *testing.T) (string, *atomic.Int64) {
	t.Helper()
	requests := &atomic.Int64{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, "outbound HTTP disabled for cache acceptance", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	return server.URL, requests
}

func (l *lab) port(t *testing.T) int {
	t.Helper()
	if l.ports == nil {
		var err error
		l.ports, err = evidence.ListenerPorts()
		require(t, err)
	}
	addresses := []string{"127.0.0.1"}
	if slices.Contains(l.nodeIPs, "::1") {
		addresses = append(addresses, "::1")
	}
	for len(l.ports) != 0 {
		port := l.ports[0]
		l.ports = l.ports[1:]
		available := true
		for _, address := range addresses {
			err := probeListenerPair(address, port)
			if errors.Is(err, syscall.EADDRINUSE) {
				available = false
				break
			}
			require(t, err)
		}
		if available {
			return port
		}
	}
	t.Fatal("no free TCP/UDP listener pair in the disjoint lab port pool")
	return 0
}

func probeListenerPair(address string, port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort(address, strconv.Itoa(port)))
	if err != nil {
		return err
	}
	defer listener.Close()
	packet, err := net.ListenPacket("udp", listener.Addr().String())
	if err != nil {
		return err
	}
	return packet.Close()
}

func outboundLoopback(out map[string]any) string {
	settings, _ := out["settings"].(map[string]any)
	address, _ := settings["address"].(string)
	for _, key := range []string{"vnext", "servers"} {
		if servers, ok := settings[key].([]any); ok && len(servers) > 0 {
			server, _ := servers[0].(map[string]any)
			address, _ = server["address"].(string)
		}
	}
	if ip := net.ParseIP(address); ip != nil && ip.To4() == nil {
		return "::1"
	}
	return "127.0.0.1"
}

func holdListenerPairs(t *testing.T, ports []int) {
	t.Helper()
	for _, port := range ports {
		address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		stream, err := net.Listen("tcp4", address)
		require(t, err)
		t.Cleanup(func() { _ = stream.Close() })
		packet, err := net.ListenPacket("udp4", address)
		require(t, err)
		t.Cleanup(func() { _ = packet.Close() })
	}
}

func (l *lab) guardIPv6Listeners(t *testing.T) {
	t.Helper()
	seen, ports := map[int]bool{}, []int{}
	for _, endpoint := range l.topology.Services {
		port := endpoint.Service.Port
		if l.topology.Nodes[endpoint.Node].Address == "::1" && !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	// Occupy the IPv4 counterparts before starting the servers. A renderer that
	// silently converts both listeners and clients to IPv4 cannot pass as IPv6.
	holdListenerPairs(t, ports)
	if len(ports) != 0 {
		t.Logf("IPv4 counterparts held for %d IPv6 TCP/UDP public ports", len(ports))
	}
}

func startOrigins(t *testing.T) origins {
	t.Helper()
	value := origins{httpHits: &atomic.Int64{}, udpHits: &atomic.Int64{}}
	value.http = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		value.httpHits.Add(1)
		_, _ = w.Write(payload)
	}))
	t.Cleanup(value.http.Close)
	return startUDPEcho(t, value, "127.0.0.1:0")
}

func startUDPEcho(t *testing.T, value origins, address string) origins {
	t.Helper()
	if value.udpHits == nil {
		value.udpHits = &atomic.Int64{}
	}
	peer, err := net.ResolveUDPAddr("udp4", address)
	require(t, err)
	value.udp, err = net.ListenUDP("udp4", peer)
	require(t, err)
	done := make(chan struct{})
	go func() { serveUDP(value); close(done) }()
	t.Cleanup(func() { _ = value.udp.Close(); <-done })
	return value
}

func checkUDPEcho(t *testing.T, origin origins) {
	t.Helper()
	peer := *origin.udp.LocalAddr().(*net.UDPAddr)
	if peer.IP.IsUnspecified() {
		peer.IP = net.IPv4(198, 18, 0, 2)
	}
	connection, err := net.DialUDP("udp4", nil, &peer)
	require(t, err)
	defer connection.Close()
	require(t, connection.SetDeadline(time.Now().Add(time.Second)))
	_, err = connection.Write([]byte("canary"))
	require(t, err)
	buffer := make([]byte, 64)
	size, err := connection.Read(buffer)
	require(t, err)
	if !bytes.Equal(buffer[:size], []byte("canary")) {
		t.Fatal("UDP canary did not echo its positive control")
	}
}

func serveUDP(origin origins) {
	buffer := make([]byte, 65535)
	for {
		length, address, err := origin.udp.ReadFromUDP(buffer)
		if err != nil {
			return
		}
		origin.udpHits.Add(1)
		_, _ = origin.udp.WriteToUDP(buffer[:length], address)
	}
}

// The TLS cover server and TCP/UDP origins are local network fixtures owned by the lab.
func donorCertificate(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require(t, err)
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1),
		Subject: pkix.Name{CommonName: "donor.example.invalid"}, DNSNames: []string{"donor.example.invalid"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, key.Public(), key)
	require(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func donor(t *testing.T) string { return startDonor(t, donorCertificate(t)) }

func startDonor(t *testing.T, certificate tls.Certificate) string {
	t.Helper()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("cover")) }))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, CurvePreferences: []tls.CurveID{tls.X25519},
		Certificates: []tls.Certificate{certificate}}
	server.EnableHTTP2 = true
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	t.Cleanup(server.Close)
	return server.Listener.Addr().String()
}
