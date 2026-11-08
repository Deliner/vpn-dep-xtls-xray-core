package rollout

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

type backend struct {
	nodes     map[string]wire.Node
	calls     []string
	ambiguous bool
	clock     func() time.Time
}

type fixture struct {
	m    *Manager
	b    *backend
	root string
	now  time.Time
	spec wire.RolloutSpec
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require(t, err)
	require(t, os.Chmod(root, 0700))
	f := &fixture{root: root, now: time.Unix(1_800_000_000, 0)}
	f.b = &backend{nodes: map[string]wire.Node{}, clock: func() time.Time { return f.now }}
	f.reopen(t)
	f.spec = wire.RolloutSpec{Version: 1, ID: "release-a", ObserveSeconds: 10, FreshSeconds: 30, StageTimeoutSeconds: 90, ValiditySeconds: 3600}
	for _, name := range []string{"canary", "second", "third"} {
		f.b.nodes[name] = wire.Node{FleetID: "test-fleet", NodeID: name, Group: "main", Enrolled: true}
		meta, err := f.m.Stage(name, manifestBytes(t, name))
		require(t, err)
		f.spec.Stages = append(f.spec.Stages, wire.RolloutStage{Name: name, Targets: []wire.RolloutTarget{{NodeID: name, CandidateSHA256: meta.SHA256}}})
	}
	return f
}

func (f *fixture) reopen(t *testing.T) {
	t.Helper()
	if f.m != nil {
		require(t, f.m.Close())
	}
	var err error
	f.m, err = Open(f.root, f.b)
	require(t, err)
	f.m.now = func() time.Time { return f.now }
	current := f.m
	t.Cleanup(func() { require(t, current.Close()) })
}

func (b *backend) Node(id string) (wire.Node, error) {
	node, ok := b.nodes[id]
	if !ok {
		return wire.Node{}, registry.ErrNotFound
	}
	return node, nil
}

func (b *backend) Publish(id string, request wire.Publish) (wire.Desired, error) {
	node, err := b.Node(id)
	if err != nil {
		return wire.Desired{}, err
	}
	if node.Desired.Revision != request.ExpectedRevision {
		return wire.Desired{}, registry.ErrConflict
	}
	data, err := manifest.Decode(request.Manifest)
	if err != nil {
		return wire.Desired{}, err
	}
	node.Desired = wire.Desired{Revision: request.ExpectedRevision + 1, Generation: data.Generation,
		ManifestSHA256: safeio.Hash(request.Manifest), RequestID: request.RequestID, PublicationID: request.RequestID,
		IssuedAt: b.clock().Unix(), ExpiresAt: b.clock().Unix() + request.ValiditySeconds}
	b.nodes[id] = node
	b.calls = append(b.calls, id)
	if b.ambiguous {
		b.ambiguous = false
		return wire.Desired{}, errors.New("simulated error after assignment commit")
	}
	return node.Desired, nil
}

func manifestBytes(t *testing.T, id string) []byte {
	t.Helper()
	data := model.NodeConfig{SchemaVersion: 1, TopologyID: "fixture", NodeID: id, Firewall: "preserve", Runtime: xray.Runtime(),
		Listeners: []model.Listener{{Tag: "in", Address: "127.0.0.1", Port: 24443, Network: "tcp"}}}
	data.Xray.Inbounds = []model.Object{{"protocol": "vless", "tag": "in", "listen": "127.0.0.1", "port": 24443}}
	data.Xray.Outbounds = []model.Object{xray.Block()}
	var err error
	data.Generation, err = safeio.Digest(data)
	require(t, err)
	raw, err := safeio.FormattedJSON(data)
	require(t, err)
	return raw
}

func (f *fixture) ready(id string) {
	node := f.b.nodes[id]
	attempt := uint64(1)
	if node.Status != nil {
		attempt = node.Status.Attempt + 1
	}
	node.Status = &wire.Status{FleetID: node.FleetID, NodeID: id, Phase: "ready", Attempt: attempt,
		AcceptedRevision: node.Desired.Revision, AppliedRevision: node.Desired.Revision, Generation: node.Desired.Generation, AttemptedAt: f.now.Unix()}
	node.LastSeen = f.now.Unix()
	node.StatusReceivedAt = f.now.Unix()
	f.b.nodes[id] = node
}

func (f *fixture) tick(t *testing.T) wire.Rollout {
	t.Helper()
	require(t, f.m.Tick(t.Context()))
	value, err := f.m.Get(f.spec.ID)
	require(t, err)
	return value
}

func (f *fixture) passStage(t *testing.T, name string) wire.Rollout {
	t.Helper()
	f.tick(t)
	f.ready(name)
	f.tick(t)
	f.now = f.now.Add(10 * time.Second)
	for id, node := range f.b.nodes {
		if node.Desired.Revision > 0 {
			f.ready(id)
		}
	}
	return f.tick(t)
}
