package masque

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/xray"
)

func setup(t *testing.T) *fixture {
	t.Helper()
	requireIsolation(t)
	directory, err := filepath.Abs(os.Getenv("NHOP_MASQUE_REPORT"))
	if err != nil {
		t.Fatal(err)
	}
	if err := safeio.CheckAccess(directory, os.ModeDir|0700, 0, 0); err != nil {
		t.Fatal(err)
	}
	// Registered before children: evidence reads only closed, complete process logs.
	t.Cleanup(func() { attributeQueueDrops(t, directory) })
	f := &fixture{directory: directory, binary: os.Getenv("XRAY_BINARY")}
	service := model.Service{Protocol: "masque-h3", Listen: "198.51.100.2", Port: 24443, ServerName: "masque.example.invalid"}
	secret, err := xray.NewSecret(service.Protocol)
	if err != nil {
		t.Fatal(err)
	}
	if err := credentials.EnsureTLS(service, &secret); err != nil {
		t.Fatal(err)
	}
	server := model.Xray{Log: model.Object{"loglevel": "info"}, Inbounds: []model.Object{xray.Inbound("masque", service, secret)},
		Outbounds: []model.Object{{"protocol": "freedom", "settings": model.Object{"finalRules": []model.Object{{
			"action": "allow", "network": "tcp,udp", "ip": []string{"198.18.0.2/32"}, "port": "18080,18081"}}}}}}
	writeJSON(t, filepath.Join(directory, "server.json"), server)
	client := xray.Client(xray.Remote("exit", "198.51.100.2", service, secret), socksPort)
	client.Log = model.Object{"loglevel": "info"}
	writeJSON(t, filepath.Join(directory, "client.json"), client)
	startOrigins(t, directory)
	f.restart(t, "server")
	f.restart(t, "client")
	return f
}

func (f *fixture) restart(t *testing.T, role string) {
	t.Helper()
	child, namespace := &f.client, ""
	if role == "server" {
		child, namespace = &f.server, exitNamespace
	}
	if *child != nil {
		(*child).stop(t)
	}
	f.sequence++
	*child = startManaged(t, f.directory, fmt.Sprintf("%s-%03d", role, f.sequence), namespace,
		f.binary, "run", "-c", filepath.Join(f.directory, role+".json"))
	if role == "client" {
		waitSOCKS(t, *child)
	}
}

func waitSOCKS(t *testing.T, child *managed) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", "127.0.0.1:10800", 100*time.Millisecond)
		if err == nil {
			connection.Close()
			return
		}
		select {
		case err := <-child.done:
			t.Fatalf("client exited before readiness: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatal("SOCKS listener did not become ready")
}
