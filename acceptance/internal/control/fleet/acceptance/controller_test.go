package acceptance

import (
	"context"
	"crypto/tls"
	"encoding/pem"
	"net"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/server"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/fleet/pull"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/fleetapi"
	"vpn-script/internal/platform/safeio"
)

func startController(t *testing.T, root string) *controller {
	t.Helper()
	c := &controller{root: filepath.Join(root, "controller")}
	require(t, registry.Initialize(c.root, "acceptance"))
	var err error
	c.store, err = registry.Open(c.root)
	require(t, err)
	c.server = httptest.NewTLSServer(server.New(c.store, nil))
	c.address, c.certificate = c.server.Listener.Addr().String(), c.server.TLS.Certificates[0]
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.server.Certificate().Raw})
	c.client, err = fleetapi.New(c.server.URL, ca)
	require(t, err)
	c.admin, err = registry.ReadAdminCredential(filepath.Join(c.root, "admin-credential.json"))
	require(t, err)
	t.Cleanup(func() { c.close(t) })
	return c
}

func (c *controller) close(t *testing.T) {
	t.Helper()
	c.client.Close()
	c.server.Close()
	require(t, c.store.Close())
}

func (c *controller) reopen(t *testing.T) {
	t.Helper()
	var err error
	c.store, err = registry.Open(c.root)
	require(t, err)
	listener, err := net.Listen("tcp", c.address)
	require(t, err)
	c.server = httptest.NewUnstartedServer(server.New(c.store, nil))
	require(t, c.server.Listener.Close())
	c.server.Listener = listener
	c.server.TLS = &tls.Config{Certificates: []tls.Certificate{c.certificate}, MinVersion: tls.VersionTLS13}
	c.server.StartTLS()
}

func (f *fixture) enroll(t *testing.T) {
	t.Helper()
	c := f.controller
	invite, err := c.client.Invite(t.Context(), c.admin, wire.Invite{NodeID: nodeID, Group: "canary", ValiditySeconds: 600})
	require(t, err)
	bootstrap := pull.Bootstrap{Version: 1, Endpoint: c.server.URL, Invitation: invite,
		CAPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.certificate.Certificate[0]})}
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
