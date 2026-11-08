package acceptance

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/credentials"
)

type dnsOrigins struct {
	direct   atomic.Uint64
	tunneled atomic.Uint64
}

func startDNSOrigins(t *testing.T) *dnsOrigins {
	t.Helper()
	origins := &dnsOrigins{}
	startDirectDNS(t, origins)
	secret := model.Secret{}
	if err := credentials.EnsureTLS(model.Service{Protocol: "trojan", ServerName: "dns.example.invalid"}, &secret); err != nil {
		t.Fatal(err)
	}
	privateRoot(t, secret.Certificate)
	certificate, err := tls.X509KeyPair([]byte(secret.Certificate), []byte(secret.CertificateKey))
	if err != nil {
		t.Fatal(err)
	}
	listener, err := tls.Listen("tcp", "198.18.0.2:443", &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(origins.serveHTTPS), ReadHeaderTimeout: time.Second}
	t.Cleanup(func() { server.Close() })
	go server.Serve(listener)
	return origins
}

func privateRoot(t *testing.T, certificate string) {
	t.Helper()
	actual, actualErr := os.Stat("/etc")
	copy, copyErr := os.Stat("/run/nhop-session-test/etc")
	if actualErr != nil || copyErr != nil || !os.SameFile(actual, copy) {
		t.Fatal("DNS fixture requires its private /etc mount")
	}
	path := "/etc/ssl/certs/ca-certificates.crt"
	if err := safeio.CheckAccess(path, 0644, 0, 0); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString("\n" + certificate); err != nil {
		t.Fatal(err)
	}
}

func startDirectDNS(t *testing.T, origins *dnsOrigins) {
	t.Helper()
	listener, err := net.ListenPacket("udp", "198.51.100.7:53")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		var data [4096]byte
		for {
			size, peer, err := listener.ReadFrom(data[:])
			if err != nil {
				return
			}
			origins.direct.Add(1)
			if answer, err := dnsReply(data[:size]); err == nil {
				listener.WriteTo(answer, peer)
			}
		}
	}()
}

func (d *dnsOrigins) serveHTTPS(writer http.ResponseWriter, request *http.Request) {
	peer, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil || peer != "198.18.0.2" {
		http.Error(writer, "resolver was reached without the test exit", http.StatusForbidden)
		return
	}
	var query []byte
	if request.Method == http.MethodGet {
		query, err = base64.RawURLEncoding.DecodeString(request.URL.Query().Get("dns"))
	} else {
		query, err = io.ReadAll(io.LimitReader(request.Body, 4097))
	}
	if err != nil || len(query) > 4096 || request.URL.Path != "/dns-query" {
		http.Error(writer, "invalid synthetic DNS request", http.StatusBadRequest)
		return
	}
	answer, err := dnsReply(query)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusBadRequest)
		return
	}
	d.tunneled.Add(1)
	writer.Header().Set("Content-Type", "application/dns-message")
	_, _ = writer.Write(answer)
}

// dnsReply implements only the bounded, single-question IN/A fixture contract.
// It drops EDNS additions before appending the answer in the correct DNS section.
func dnsReply(query []byte) ([]byte, error) {
	if len(query) < 17 || len(query) > 4096 || binary.BigEndian.Uint16(query[4:6]) != 1 {
		return nil, fmt.Errorf("invalid synthetic DNS query")
	}
	end := 12
	for {
		if end >= len(query) || query[end] > 63 {
			return nil, fmt.Errorf("invalid synthetic DNS name")
		}
		length := int(query[end])
		end += 1 + length
		if length == 0 {
			break
		}
	}
	if end+4 > len(query) || binary.BigEndian.Uint16(query[end:end+2]) != 1 || binary.BigEndian.Uint16(query[end+2:end+4]) != 1 {
		return nil, fmt.Errorf("expected A/IN question")
	}
	answer := append([]byte(nil), query[:end+4]...)
	answer[2], answer[3] = 0x81, 0x80
	answer[6], answer[7] = 0, 1
	for index := 8; index < 12; index++ {
		answer[index] = 0
	}
	// TTL zero and a unique name per probe prevent cached replies proving routing.
	return append(answer, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 0, 0, 4, 198, 18, 0, 2), nil
}
