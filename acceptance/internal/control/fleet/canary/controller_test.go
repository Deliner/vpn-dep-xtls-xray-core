package canary

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/safeio"
)

func (f *fixture) authority(t *testing.T) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Owned Fleet acceptance"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(4 * time.Hour),
		IPAddresses: []net.IP{net.ParseIP(f.addresses["fleet-a"]), net.IPv4(127, 0, 0, 1)},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, public, private)
	require(t, err)
	key, err := x509.MarshalPKCS8PrivateKey(private)
	require(t, err)
	for name, data := range map[string][]byte{"ca.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		"key.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})} {
		path := filepath.Join(f.root, name)
		require(t, safeio.Atomic(path, data, 0600))
		f.put(t, "fleet-a", path, name, "0600")
	}
	f.guest(t, "fleet-a", f.remote+"/nhop-fleet", "init", "--state", f.remote+"/controller", "--fleet", "three-machine-acceptance")
	f.startController(t)
	t.Cleanup(func() { f.guest(t, "fleet-a", "systemctl", "stop", controllerUnit) })
}

func (f *fixture) startController(t *testing.T) {
	t.Helper()
	f.guest(t, "fleet-a", "systemd-run", "--unit="+controllerUnit, "--collect", "--property=Type=exec",
		f.remote+"/nhop-fleet", "serve", "--state", f.remote+"/controller", "--cert", f.remote+"/ca.pem",
		"--key", f.remote+"/key.pem", "--listen", "0.0.0.0:18443")
	eventually(t, "production HTTPS controller", 15*time.Second, func() bool {
		return f.property(t, "fleet-a", controllerUnit, "SubState") == "running"
	})
}

func (f *fixture) admin(t *testing.T, command string, args ...string) []byte {
	t.Helper()
	values := []string{f.remote + "/nhop-fleet", command, "--endpoint", "https://127.0.0.1:18443",
		"--credential", f.remote + "/controller/admin-credential.json", "--ca", f.remote + "/ca.pem"}
	return f.guest(t, "fleet-a", append(values, args...)...)
}

func (f *fixture) enroll(t *testing.T) {
	t.Helper()
	for _, node := range nodes {
		name := node + "-bootstrap.json"
		f.admin(t, "invite", "--endpoint", "https://"+f.addresses["fleet-a"]+":18443", "--node", node,
			"--output", f.remote+"/"+name, "--validity", "3600")
		f.saveRemote(t, "fleet-a", f.remote+"/"+name, name)
		if node != "fleet-a" {
			f.put(t, node, filepath.Join(f.root, "fleet-a-"+name), name, "0600")
		}
		f.guest(t, node, f.remote+"/nhop-fleet", "agent-init", "--state", agentState, "--bootstrap", f.remote+"/"+name)
		f.guest(t, node, f.remote+"/nhop-fleet", "agent-service-install", "--state", agentState, "--interval", "1s")
		f.guest(t, node, f.remote+"/nhop-fleet", "agent-service-start")
	}
	eventually(t, "all three real agents enrolled", time.Minute, func() bool {
		inventory := f.inventory(t)
		if len(inventory) != 3 {
			return false
		}
		for _, node := range inventory {
			if !node.Enrolled || node.Status == nil || node.Status.Phase != "waiting" {
				return false
			}
		}
		return true
	})
}

func (f *fixture) inventory(t *testing.T) []wire.Node {
	t.Helper()
	return decode[[]wire.Node](t, f.admin(t, "nodes"))
}
