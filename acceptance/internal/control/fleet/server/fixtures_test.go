package server_test

import (
	"context"
	"encoding/pem"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout"
	"vpn-script/internal/control/fleet/server"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/fleetapi"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

type fixture struct {
	store    *registry.Store
	server   *httptest.Server
	client   *fleetapi.Client
	admin    string
	rollouts *rollout.Manager
	root     string
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	require(t, err)
	root := filepath.Join(base, "registry")
	require(t, registry.Initialize(root, "test-fleet"))
	store, err := registry.Open(root)
	require(t, err)
	t.Cleanup(func() { require(t, store.Close()) })
	coordinator, err := rollout.Open(filepath.Join(root, "rollouts"), store)
	require(t, err)
	t.Cleanup(func() { require(t, coordinator.Close()) })
	endpoint := httptest.NewTLSServer(server.New(store, coordinator))
	t.Cleanup(endpoint.Close)
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: endpoint.Certificate().Raw})
	client, err := fleetapi.New(endpoint.URL, ca)
	require(t, err)
	t.Cleanup(client.Close)
	admin, err := registry.ReadAdminCredential(filepath.Join(root, "admin-credential.json"))
	require(t, err)
	return &fixture{store: store, server: endpoint, client: client, admin: admin, rollouts: coordinator, root: root}
}

func (f *fixture) enroll(t *testing.T, node string) (wire.Invitation, string) {
	t.Helper()
	invite, err := f.client.Invite(context.Background(), f.admin, wire.Invite{NodeID: node, Group: "canary", ValiditySeconds: 600})
	require(t, err)
	token, err := credential.NewToken()
	require(t, err)
	identity, err := f.client.Enroll(context.Background(), wire.Enrollment{NodeID: node, Token: invite.Token, Credential: token})
	require(t, err)
	if identity.FleetID != invite.FleetID || identity.NodeID != node {
		t.Fatal("enrollment identity mismatch")
	}
	return invite, token
}

func publication(t *testing.T, node string) wire.Publish {
	t.Helper()
	data := model.NodeConfig{SchemaVersion: 1, TopologyID: "fixture", NodeID: node, Firewall: "preserve", Runtime: xray.Runtime(),
		Listeners: []model.Listener{{Tag: "in", Address: "127.0.0.1", Port: 24443, Network: "tcp"}}}
	data.Xray.Inbounds = []model.Object{{"protocol": "vless", "tag": "in", "listen": "127.0.0.1", "port": 24443}}
	data.Xray.Outbounds = []model.Object{xray.Block()}
	var err error
	data.Generation, err = safeio.Digest(data)
	require(t, err)
	raw, err := safeio.FormattedJSON(data)
	require(t, err)
	nonce, err := credential.NewToken()
	require(t, err)
	return wire.Publish{RequestID: credential.TokenHash(nonce), ValiditySeconds: 3600, Manifest: raw}
}

func requireHTTP(t *testing.T, err error, status int) {
	t.Helper()
	var response *fleetapi.HTTPError
	if !errors.As(err, &response) || response.Status != status {
		t.Fatalf("expected HTTP %d, got %v", status, err)
	}
}

func (f *fixture) rolloutPlan(t *testing.T) (wire.RolloutSpec, map[string]string) {
	t.Helper()
	spec := wire.RolloutSpec{Version: 1, ID: "canary-release", ObserveSeconds: 1, FreshSeconds: 30, StageTimeoutSeconds: 30, ValiditySeconds: 3600}
	tokens := map[string]string{}
	for _, name := range []string{"node-a", "node-b"} {
		_, tokens[name] = f.enroll(t, name)
		candidate, err := f.client.StageCandidate(t.Context(), name, f.admin, publication(t, name).Manifest)
		require(t, err)
		spec.Stages = append(spec.Stages, wire.RolloutStage{Name: name, Targets: []wire.RolloutTarget{{NodeID: name, CandidateSHA256: candidate.SHA256}}})
	}
	return spec, tokens
}
