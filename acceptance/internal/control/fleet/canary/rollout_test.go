package canary

import (
	"strconv"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
)

func (f *fixture) createRollout(t *testing.T, id string, configs map[string]model.NodeConfig, observe int64) wire.Rollout {
	t.Helper()
	revisions := map[string]uint64{}
	for _, node := range f.inventory(t) {
		revisions[node.NodeID] = node.Desired.Revision
	}
	spec := wire.RolloutSpec{Version: 1, ID: id, ObserveSeconds: observe, FreshSeconds: 10, StageTimeoutSeconds: 600, ValiditySeconds: 3600}
	for _, node := range nodes {
		name := id + "-" + node + ".json"
		f.putJSON(t, "fleet-a", name, configs[node])
		candidate := decode[wire.Candidate](t, f.admin(t, "candidate", "--manifest", f.remote+"/"+name))
		spec.Stages = append(spec.Stages, wire.RolloutStage{Name: node, Targets: []wire.RolloutTarget{{NodeID: node,
			CandidateSHA256: candidate.SHA256, ExpectedRevision: revisions[node]}}})
	}
	f.putJSON(t, "fleet-a", id+"-request.json", spec)
	return decode[wire.Rollout](t, f.admin(t, "rollout-create", "--request", f.remote+"/"+id+"-request.json"))
}

func (f *fixture) rollout(t *testing.T, id string) wire.Rollout {
	t.Helper()
	return decode[wire.Rollout](t, f.admin(t, "rollout", "--id", id))
}

func (f *fixture) waitRollout(t *testing.T, id, phase string) wire.Rollout {
	t.Helper()
	var value wire.Rollout
	previous := ""
	eventually(t, id+" "+phase, 12*time.Minute, func() bool {
		value = f.rollout(t, id)
		f.report.Rollouts[id] = value
		state := value.Phase + "/" + value.StageName
		if state != previous {
			t.Logf("%s: %s; published=%d ready=%d", id, state, value.PublishedNodes, value.ReadyNodes)
			previous = state
		}
		if value.Phase != "running" && value.Phase != phase {
			t.Fatalf("unexpected rollout outcome: %+v", value)
		}
		for _, node := range value.Nodes {
			if node.Stage > value.Stage && node.PublishedRevision != 0 {
				t.Fatal("later stage published ahead of the health gate")
			}
		}
		return value.Phase == phase
	})
	return value
}

func (f *fixture) firstPublication(t *testing.T, id string) wire.Rollout {
	t.Helper()
	var value wire.Rollout
	eventually(t, "first canary publication", time.Minute, func() bool {
		value = f.rollout(t, id)
		return value.PublishedNodes > 0
	})
	if value.Phase != "running" || value.PublishedNodes != 1 || value.Stage != 0 {
		t.Fatal("missed the exclusive first canary stage", value)
	}
	return value
}

func (f *fixture) waitReady(t *testing.T, id string, revision uint64) wire.Node {
	t.Helper()
	var current wire.Node
	eventually(t, id+" ready status", time.Minute, func() bool {
		for _, node := range f.inventory(t) {
			if node.NodeID == id {
				current = node
				return node.Status != nil && node.Status.Phase == "ready" && node.Status.AppliedRevision == revision
			}
		}
		return false
	})
	return current
}

func (f *fixture) recoverCanary(t *testing.T) {
	t.Helper()
	desired := f.publishNode(t, "recover", f.current["fleet-c"])
	f.guest(t, "fleet-c", f.remote+"/nhop-fleet", "agent-service-start")
	f.waitReady(t, "fleet-c", desired.Revision)
}

func (f *fixture) publishNode(t *testing.T, id string, data model.NodeConfig) wire.Desired {
	t.Helper()
	var expected uint64
	for _, node := range f.inventory(t) {
		if node.NodeID == data.NodeID {
			expected = node.Desired.Revision
		}
	}
	f.putJSON(t, "fleet-a", id+"-manifest.json", data)
	f.guest(t, "fleet-a", f.remote+"/nhop-fleet", "assignment", "--manifest", f.remote+"/"+id+"-manifest.json",
		"--expected", strconv.FormatUint(expected, 10), "--output", f.remote+"/"+id+".json", "--validity", "3600")
	return decode[wire.Desired](t, f.admin(t, "publish", "--request", f.remote+"/"+id+".json"))
}
