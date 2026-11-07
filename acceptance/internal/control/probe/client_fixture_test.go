package probe

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/control/probe/ipcheck"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/sshutil"
)

func probeRuntimeConfig(engine string) model.Object {
	if engine == "xray" {
		return model.Object{
			"log":       model.Object{"loglevel": "error"},
			"inbounds":  []any{map[string]any{"protocol": "socks", "listen": "127.0.0.1", "port": 1080, "settings": model.Object{"udp": false}}},
			"outbounds": []any{model.Object{"protocol": "freedom", "tag": "direct"}},
			"routing":   model.Object{},
		}
	}
	return model.Object{
		"log":       model.Object{"level": "error"},
		"inbounds":  []any{map[string]any{"type": "socks", "listen": "127.0.0.1", "listen_port": 1080}},
		"outbounds": []any{model.Object{"type": "direct", "tag": "direct"}},
		"route":     model.Object{"final": "direct"},
	}
}

func runtimeHTTPS(t *testing.T, port int) (*http.Client, string) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "198.51.100.42")
	}))
	t.Cleanup(server.Close)
	roots := x509.NewCertPool()
	roots.AddCert(server.Certificate())
	client := ipcheck.NewClient(port)
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	t.Cleanup(client.CloseIdleConnections)
	return client, server.URL
}

func probeHelper(t *testing.T, mode, directory string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "owned-runtime")
	script := "#!/bin/sh\nexec " + sshutil.Shell(os.Args[0], "nhop-probe-helper", mode, directory) + " \"$@\"\n"
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitProbeChild(t *testing.T, directory string) probeChild {
	t.Helper()
	for range 300 {
		raw, err := os.ReadFile(filepath.Join(directory, "ready.json"))
		var child probeChild
		if err == nil && json.Unmarshal(raw, &child) == nil && child.PID > 0 {
			t.Cleanup(func() { _ = syscall.Kill(child.PID, syscall.SIGKILL) })
			return child
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("owned runtime helper did not become ready")
	return probeChild{}
}

func assertProbeClosed(t *testing.T, address string) {
	t.Helper()
	for range 100 {
		connection, err := net.DialTimeout("tcp", address, 25*time.Millisecond)
		if err != nil {
			return
		}
		connection.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("owned runtime listener survived cleanup")
}
