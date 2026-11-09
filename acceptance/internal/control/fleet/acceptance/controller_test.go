package acceptance

import (
	"context"
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
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/fleet/pull"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/safeio"
)

func (c *controller) authority(t *testing.T) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Owned Fleet acceptance"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(4 * time.Hour),
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, public, private)
	require(t, err)
	key, err := x509.MarshalPKCS8PrivateKey(private)
	require(t, err)
	c.ca = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	for name, raw := range map[string][]byte{"ca.pem": c.ca, "key.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})} {
		require(t, safeio.Atomic(filepath.Join(c.root, name), raw, 0600))
	}
}

func (f *fixture) enroll(t *testing.T) {
	t.Helper()
	c := f.controller
	invite, err := c.client.Invite(t.Context(), c.admin, wire.Invite{NodeID: nodeID, Group: "canary", ValiditySeconds: 600})
	require(t, err)
	bootstrap := pull.Bootstrap{Version: 1, Endpoint: c.endpoint, Invitation: invite, CAPEM: c.ca}
	path := filepath.Join(f.root, "bootstrap.json")
	require(t, safeio.NewJSON(path, bootstrap))
	f.cli(t, "agent-init", "--state", agentState, "--bootstrap", path)
	f.cli(t, "agent-service-install", "--state", agentState, "--interval", "1s")
	f.cli(t, "agent-service-start")
	f.waitStatus(t, "waiting", 0)
}

func (f *fixture) publish(t *testing.T, data model.NodeConfig) uint64 {
	t.Helper()
	c := f.controller
	node, err := c.client.Node(t.Context(), nodeID, c.admin)
	require(t, err)
	raw, err := safeio.FormattedJSON(data)
	require(t, err)
	nonce, err := credential.NewToken()
	require(t, err)
	desired, err := c.client.Publish(t.Context(), nodeID, c.admin, wire.Publish{RequestID: credential.TokenHash(nonce),
		ExpectedRevision: node.Desired.Revision, ValiditySeconds: 3600, Manifest: raw})
	require(t, err)
	return desired.Revision
}

func (f *fixture) waitStatus(t *testing.T, phase string, revision uint64) wire.Node {
	t.Helper()
	var observed wire.Node
	eventually(t, "node "+phase, 6*time.Minute, func() bool {
		var err error
		observed, err = f.controller.client.Node(context.Background(), nodeID, f.controller.admin)
		return err == nil && observed.Status != nil && observed.Status.Phase == phase && observed.Status.AcceptedRevision == revision
	})
	return observed
}
