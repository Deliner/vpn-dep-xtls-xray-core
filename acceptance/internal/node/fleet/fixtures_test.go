package fleet

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"path/filepath"
	"testing"
	"time"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

type agentFixture struct {
	agent *Agent
	key   ed25519.PrivateKey
	data  model.NodeConfig
	now   time.Time
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newAgentFixture(t *testing.T) agentFixture {
	t.Helper()
	key := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{72}, ed25519.SeedSize))
	now := time.Unix(1_800_000_000, 0)
	trust := contract.Trust{FleetID: "owned-fleet", NodeID: "entry", PublicKey: key.Public().(ed25519.PublicKey)}
	agent := New(filepath.Join(t.TempDir(), "agent"), trust)
	agent.now = func() time.Time { return now }
	agent.install = func(_ context.Context, data model.NodeConfig) (installer.Result, error) {
		return installed(data), nil
	}
	require(t, agent.Initialize())
	return agentFixture{agent: agent, key: key, now: now, data: nodeManifest(t)}
}

func nodeManifest(t *testing.T) model.NodeConfig {
	t.Helper()
	data := model.NodeConfig{SchemaVersion: 1, TopologyID: "fixture", NodeID: "entry", Firewall: "preserve", Runtime: xray.Runtime(),
		Listeners: []model.Listener{{Tag: "in", Address: "127.0.0.1", Port: 24443, Network: "tcp"}}}
	data.Xray.Inbounds = []model.Object{{"protocol": "vless", "tag": "in", "listen": "127.0.0.1", "port": 24443}}
	data.Xray.Outbounds = []model.Object{xray.Block()}
	return regenerate(t, data)
}

func regenerate(t *testing.T, data model.NodeConfig) model.NodeConfig {
	t.Helper()
	data.Generation = ""
	var err error
	data.Generation, err = safeio.Digest(data)
	require(t, err)
	return data
}

func installed(data model.NodeConfig) installer.Result {
	return installer.Result{Node: data.NodeID, Generation: data.Generation, Result: "applied"}
}

func (f agentFixture) signed(t *testing.T, revision uint64) []byte {
	t.Helper()
	raw, err := safeio.FormattedJSON(f.data)
	require(t, err)
	value, err := contract.NewAssignment(f.agent.trust.FleetID, f.agent.trust.NodeID, revision, raw, f.now, time.Hour)
	require(t, err)
	raw, err = contract.Sign(value, f.key, f.now)
	require(t, err)
	return raw
}

func (f agentFixture) restarted(t *testing.T) *Agent {
	t.Helper()
	agent := New(f.agent.directory, f.agent.trust)
	agent.now, agent.install = f.agent.now, f.agent.install
	require(t, agent.Initialize())
	return agent
}
