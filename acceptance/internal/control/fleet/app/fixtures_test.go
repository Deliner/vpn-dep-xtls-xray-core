package app

import (
	"bytes"
	"context"
	"encoding/pem"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout"
	"vpn-script/internal/control/fleet/server"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/fleetapi"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

type apiFixture struct {
	root    string
	client  *fleetapi.Client
	common  []string
	command func(...string) []byte
}

func newAPIFixture(t *testing.T) *apiFixture {
	t.Helper()
	root := privateDirectory(t)
	state := filepath.Join(root, "registry")
	command := func(args ...string) []byte {
		var output bytes.Buffer
		require(t, Run(context.Background(), args, &output))
		return output.Bytes()
	}
	command("init", "--state", state, "--fleet", "test-fleet")
	store, err := registry.Open(state)
	require(t, err)
	t.Cleanup(func() { require(t, store.Close()) })
	coordinator, err := rollout.Open(filepath.Join(state, "rollouts"), store)
	require(t, err)
	t.Cleanup(func() { require(t, coordinator.Close()) })
	endpoint := httptest.NewTLSServer(server.New(store, coordinator))
	t.Cleanup(endpoint.Close)
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: endpoint.Certificate().Raw})
	caPath := filepath.Join(root, "ca.pem")
	require(t, safeio.Create(caPath, ca, 0644))
	client, err := fleetapi.New(endpoint.URL, ca)
	require(t, err)
	t.Cleanup(client.Close)
	common := []string{"--endpoint", endpoint.URL, "--credential", filepath.Join(state, "admin-credential.json"), "--ca", caPath}
	return &apiFixture{root: root, client: client, common: common, command: command}
}

func (f *apiFixture) admin(command string, flags ...string) []byte {
	args := append([]string{command}, f.common...)
	return f.command(append(args, flags...)...)
}

func manifestFile(t *testing.T, root string) string {
	return nodeManifestFile(t, root, "node-a")
}

func nodeManifestFile(t *testing.T, root, node string) string {
	t.Helper()
	data := model.NodeConfig{SchemaVersion: 1, TopologyID: "fixture", NodeID: node, Firewall: "preserve", Runtime: xray.Runtime(),
		Listeners: []model.Listener{{Tag: "in", Address: "127.0.0.1", Port: 24443, Network: "tcp"}}}
	data.Xray.Inbounds = []model.Object{{"protocol": "vless", "tag": "in", "listen": "127.0.0.1", "port": 24443}}
	data.Xray.Outbounds = []model.Object{xray.Block()}
	var err error
	data.Generation, err = safeio.Digest(data)
	require(t, err)
	path := filepath.Join(root, node+".json")
	require(t, safeio.NewJSON(path, data))
	return path
}
