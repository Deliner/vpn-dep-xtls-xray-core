package paneltest

import (
	"encoding/json"
	"testing"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

func (f *fixture) enroll(t *testing.T, node string) string {
	t.Helper()
	invite, err := f.store.Invite(wire.Invite{NodeID: node, Group: "canary", ValiditySeconds: 600})
	require(t, err)
	token, err := credential.NewToken()
	require(t, err)
	_, err = f.store.Enroll(wire.Enrollment{NodeID: node, Token: invite.Token, Credential: token})
	require(t, err)
	return token
}

func publication(t *testing.T, id string) wire.Publish {
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
	nonce, err := credential.NewToken()
	require(t, err)
	return wire.Publish{RequestID: credential.TokenHash(nonce), ValiditySeconds: 3600, Manifest: raw}
}

func encoded(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	require(t, err)
	return raw
}

func (f *fixture) stage(t *testing.T, kind string, request any) string {
	t.Helper()
	response, _ := f.send(t, f.form(t, "/panel/prepare/"+kind, nil, map[string][]byte{"request": encoded(t, request)}))
	if response.StatusCode != 303 {
		t.Fatalf("review staging status %d", response.StatusCode)
	}
	return response.Header.Get("Location")
}

func (f *fixture) plan(t *testing.T) wire.RolloutSpec {
	t.Helper()
	spec := wire.RolloutSpec{Version: 1, ID: "canary-release", ObserveSeconds: 1, FreshSeconds: 30, StageTimeoutSeconds: 30, ValiditySeconds: 3600}
	for _, name := range []string{"node-a", "node-b"} {
		f.enroll(t, name)
		candidate, err := f.rollouts.Stage(name, publication(t, name).Manifest)
		require(t, err)
		spec.Stages = append(spec.Stages, wire.RolloutStage{Name: name, Targets: []wire.RolloutTarget{{NodeID: name, CandidateSHA256: candidate.SHA256}}})
	}
	return spec
}
