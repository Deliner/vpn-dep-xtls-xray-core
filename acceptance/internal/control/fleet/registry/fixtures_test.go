package registry

import (
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

type fixture struct {
	store *Store
	root  string
	now   time.Time
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{root: filepath.Join(t.TempDir(), "registry"), now: time.Unix(1_800_000_000, 0)}
	require(t, Initialize(f.root, "owned-fleet"))
	f.reopen(t)
	return f
}

func (f *fixture) reopen(t *testing.T) {
	t.Helper()
	if f.store != nil {
		require(t, f.store.Close())
	}
	var err error
	f.store, err = Open(f.root)
	require(t, err)
	f.store.now = func() time.Time { return f.now }
	current := f.store
	t.Cleanup(func() { require(t, current.Close()) })
}

func (f *fixture) enroll(t *testing.T, nodeID string) (wire.Invitation, string) {
	t.Helper()
	invite, err := f.store.Invite(wire.Invite{NodeID: nodeID, Group: "main", ValiditySeconds: 600})
	require(t, err)
	token, err := credential.NewToken()
	require(t, err)
	_, err = f.store.Enroll(wire.Enrollment{NodeID: nodeID, Token: invite.Token, Credential: token})
	require(t, err)
	return invite, token
}

func publication(t *testing.T, nodeID string, expected uint64) wire.Publish {
	t.Helper()
	data := model.NodeConfig{SchemaVersion: 1, TopologyID: "fixture", NodeID: nodeID, Firewall: "preserve", Runtime: xray.Runtime(),
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
	return wire.Publish{RequestID: credential.TokenHash(nonce), ExpectedRevision: expected, ValiditySeconds: 3600, Manifest: raw}
}

func readyReport(f *fixture, nodeID string, desired wire.Desired, attempt uint64) wire.Status {
	return wire.Status{FleetID: "owned-fleet", NodeID: nodeID, Phase: "ready", Attempt: attempt,
		AcceptedRevision: desired.Revision, AppliedRevision: desired.Revision, Generation: desired.Generation, AttemptedAt: f.now.Unix()}
}
