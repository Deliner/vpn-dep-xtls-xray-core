package fleetapi

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func trustedClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	client, err := New(server.URL, ca)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestRejectsNonOriginEndpoints(t *testing.T) {
	for _, endpoint := range []string{"http://example.org", "https://u:p@example.org", "https://example.org/x", "https://example.org?", "https://example.org?q=x", "https://example.org/#fragment", "https://example.org:0", "https://example.org:65536", "https://example.org:", "https://example.org/%2f"} {
		if client, err := New(endpoint, nil); err == nil {
			client.Close()
			t.Errorf("accepted %q", endpoint)
		}
	}
	if _, err := New("https://example.org", []byte("invalid CA")); err == nil {
		t.Fatal("invalid private CA accepted")
	}
}

func TestRequiresTLS13(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.TLS = &tls.Config{MaxVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	client := trustedClient(t, server)
	if _, err := client.Assignment(context.Background(), "node-a", ""); err == nil {
		t.Fatal("TLS 1.2 accepted")
	}
}

func TestRefusesUntrustedCertificate(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer server.Close()
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	trusted := trustedClient(t, server)
	if _, err := trusted.Assignment(context.Background(), "node-a", ""); err != nil {
		t.Fatal(err)
	}
	untrusted, err := New(server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer untrusted.Close()
	if _, err := untrusted.Assignment(context.Background(), "node-a", ""); err == nil {
		t.Fatal("untrusted TLS accepted")
	}
}

func TestNeverFollowsCredentialRedirects(t *testing.T) {
	var received atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received.Add(1) }))
	defer target.Close()
	source := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/stolen", http.StatusTemporaryRedirect)
	}))
	defer source.Close()
	client := trustedClient(t, source)
	_, err := client.Assignment(context.Background(), "node-a", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA")
	if response, ok := err.(*HTTPError); !ok || response.Status != http.StatusTemporaryRedirect || received.Load() != 0 {
		t.Fatalf("redirect policy failed: %v, target requests %d", err, received.Load())
	}
}
