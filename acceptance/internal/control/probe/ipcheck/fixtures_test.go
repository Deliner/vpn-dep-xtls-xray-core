package ipcheck

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/runtime/credentials"
)

type ipFixture struct {
	endpoint    string
	port        int
	observed    <-chan string
	roots       *x509.CertPool
	certificate string
}

func probeHTTPFixture(t *testing.T, handler http.Handler) ipFixture {
	t.Helper()
	var secret model.Secret
	service := model.Service{Protocol: "trojan", ServerName: "ip-probe.invalid"}
	if err := credentials.EnsureTLS(service, &secret); err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair([]byte(secret.Certificate), []byte(secret.CertificateKey))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	t.Cleanup(server.Close)
	port, observed := probeSOCKS(t, server.Listener.Addr().String())
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM([]byte(secret.TrustCertificate))
	endpoint := strings.Replace(server.URL, "127.0.0.1", "ip-probe.invalid", 1)
	return ipFixture{endpoint: endpoint, port: port, observed: observed, roots: roots, certificate: secret.TrustCertificate}
}

func hostileCurlConfig(t *testing.T, fixture ipFixture) {
	t.Helper()
	directory := t.TempDir()
	config := "output = /dev/null\nwrite-out = 198.51.100.99\n"
	if err := os.WriteFile(filepath.Join(directory, ".curlrc"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	certificate := filepath.Join(directory, "origin.pem")
	if err := os.WriteFile(certificate, []byte(fixture.certificate), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CURL_HOME", directory)
	t.Setenv("CURL_CA_BUNDLE", certificate)
}

func trustedIPClient(t *testing.T, fixture ipFixture) *http.Client {
	t.Helper()
	client := NewClient(fixture.port)
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: fixture.roots, MinVersion: tls.VersionTLS12}
	t.Cleanup(client.CloseIdleConnections)
	return client
}
