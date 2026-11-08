package planning

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

type observer struct {
	nodes          map[string]wire.Node
	reads, uploads int
	changedPath    string
}

func (r *observer) Node(ctx context.Context, id, token string) (wire.Node, error) {
	if err := ctx.Err(); err != nil {
		return wire.Node{}, err
	}
	r.reads++
	if r.changedPath != "" {
		if err := os.WriteFile(r.changedPath, []byte("replaced during snapshot"), 0600); err != nil {
			return wire.Node{}, err
		}
	}
	return r.nodes[id], nil
}

func (r *observer) StageCandidate(ctx context.Context, id, token string, raw []byte) (wire.Candidate, error) {
	r.uploads++
	data, err := manifest.Decode(raw)
	return wire.Candidate{NodeID: id, SHA256: safeio.Hash(raw), Generation: data.Generation}, err
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func planFixture(t *testing.T) (string, *observer) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require(t, err)
	require(t, os.Chmod(root, 0700))
	remote := &observer{nodes: map[string]wire.Node{}}
	value := draft{Version: 1, ID: "release-a", ObserveSeconds: 10, FreshSeconds: 30, StageTimeoutSeconds: 60, ValiditySeconds: 3600}
	for _, name := range []string{"canary", "later"} {
		writeManifest(t, root, name)
		remote.nodes[name] = wire.Node{FleetID: "test-fleet", NodeID: name, Enrolled: true, Desired: wire.Desired{Revision: 7}}
		value.Stages = append(value.Stages, stage{Name: name, Manifests: []string{name + ".json"}})
	}
	raw, err := yaml.Marshal(value)
	require(t, err)
	path := filepath.Join(root, "rollout.yaml")
	require(t, safeio.Create(path, raw, 0600))
	return path, remote
}

func writeManifest(t *testing.T, root, name string) {
	t.Helper()
	data := model.NodeConfig{SchemaVersion: 1, TopologyID: "fixture", NodeID: name, Firewall: "preserve", Runtime: xray.Runtime(),
		Listeners: []model.Listener{{Tag: "in", Address: "127.0.0.1", Port: 24443, Network: "tcp"}}}
	data.Xray.Inbounds = []model.Object{{"protocol": "vless", "tag": "in", "listen": "127.0.0.1", "port": 24443}}
	data.Xray.Outbounds = []model.Object{xray.Block()}
	var err error
	data.Generation, err = safeio.Digest(data)
	require(t, err)
	require(t, safeio.NewJSON(filepath.Join(root, name+".json"), data))
}

func TestPreparedYAMLKeepsStageOrderExactBytesAndObservedRevisions(t *testing.T) {
	path, remote := planFixture(t)
	value, err := Prepare(t.Context(), remote, "synthetic", path)
	require(t, err)
	if len(value.Stages) != 2 || remote.reads != 2 || remote.uploads != 2 {
		t.Fatal("plan did not stage both inspected candidates")
	}
	for index, name := range []string{"canary", "later"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(path), name+".json"))
		require(t, err)
		target := value.Stages[index].Targets[0]
		if target.NodeID != name || target.ExpectedRevision != 7 || target.CandidateSHA256 != safeio.Hash(raw) {
			t.Fatal("prepared request differs from ordered local manifests or observed registry")
		}
	}
}

func TestInvalidLaterManifestFailsBeforeAnyNetworkOperation(t *testing.T) {
	path, remote := planFixture(t)
	require(t, os.WriteFile(filepath.Join(filepath.Dir(path), "later.json"), []byte("bad manifest"), 0600))
	if _, err := Prepare(t.Context(), remote, "synthetic", path); err == nil || remote.reads != 0 || remote.uploads != 0 {
		t.Fatal("invalid later input was discovered after contacting the controller")
	}
}

func TestLocalReplacementAndUnenrolledNodesPreventStaging(t *testing.T) {
	for _, failure := range []string{"changed", "unenrolled", "canceled"} {
		t.Run(failure, func(t *testing.T) {
			path, remote := planFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch failure {
			case "changed":
				remote.changedPath = filepath.Join(filepath.Dir(path), "canary.json")
			case "unenrolled":
				node := remote.nodes["later"]
				node.Enrolled = false
				remote.nodes["later"] = node
			case "canceled":
				cancel()
			}
			if _, err := Prepare(ctx, remote, "synthetic", path); err == nil || remote.uploads != 0 {
				t.Fatal("unsafe snapshot uploaded candidates")
			}
		})
	}
}

func TestRolloutYAMLRejectsAmbiguousAndPublicInputs(t *testing.T) {
	for _, extra := range []string{"unknown: true\n", "---\nversion: 1\n", "extra: &shared hello\n", "extra: null\n", "public"} {
		t.Run(extra, func(t *testing.T) {
			path, remote := planFixture(t)
			raw, err := os.ReadFile(path)
			require(t, err)
			if extra == "public" {
				require(t, os.Chmod(path, 0644))
			} else {
				require(t, os.WriteFile(path, append(raw, []byte(extra)...), 0600))
			}
			if _, err := Prepare(t.Context(), remote, "synthetic", path); err == nil || remote.reads != 0 || remote.uploads != 0 {
				t.Fatal("ambiguous or public YAML reached the controller")
			}
		})
	}
}
