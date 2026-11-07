package network

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/runtime/credentials"
)

var dnsQuestion = []byte{0x33, 0x33, 1, 0, 0, 1, 0, 0, 0, 0, 0, 0, 6, 't', 'u', 'n', 'n', 'e', 'l', 4, 't', 'e', 's', 't', 0, 0, 1, 0, 1}
var dnsSequence atomic.Uint64

func probeDNS(address string) bool {
	connection, err := net.DialTimeout("udp", address, time.Second)
	if err != nil {
		return false
	}
	defer connection.Close()
	if connection.SetDeadline(time.Now().Add(time.Second)) != nil {
		return false
	}
	// Distinct names exercise the DNS path after failure instead of a cached reply.
	question := append([]byte(nil), dnsQuestion...)
	copy(question[13:19], fmt.Sprintf("%06d", dnsSequence.Add(1)))
	if _, err := connection.Write(question); err != nil {
		return false
	}
	var buffer [4096]byte
	n, err := connection.Read(buffer[:])
	return err == nil && n >= 16 && binary.BigEndian.Uint16(buffer[:2]) == 0x3333 &&
		buffer[3]&15 == 0 && string(buffer[n-4:n]) == string([]byte{198, 18, 0, 2})
}

func startTunneledDNS(t *testing.T) string {
	t.Helper()
	secret := model.Secret{}
	if err := credentials.EnsureTLS(model.Service{Protocol: "trojan", ServerName: "dns.example.invalid"}, &secret); err != nil {
		t.Fatal(err)
	}
	certificate, err := tls.X509KeyPair([]byte(secret.Certificate), []byte(secret.CertificateKey))
	if err != nil {
		t.Fatal(err)
	}
	listener, err := tls.Listen("tcp", "198.18.0.2:443", &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(answerTunneledDNS), ReadHeaderTimeout: time.Second}
	t.Cleanup(func() { server.Close() })
	go server.Serve(listener)
	return secret.Certificate
}

func answerTunneledDNS(writer http.ResponseWriter, request *http.Request) {
	query, err := io.ReadAll(io.LimitReader(request.Body, 4096))
	if err != nil || len(query) < 12 || len(query) > 512 {
		http.Error(writer, "invalid synthetic DNS question", http.StatusBadRequest)
		return
	}
	query[2], query[3], query[6], query[7] = 0x81, 0x80, 0, 1
	answer := append(query, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4, 198, 18, 0, 2)
	writer.Header().Set("Content-Type", "application/dns-message")
	writer.Write(answer)
}
