package rollout

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/platform/safeio"
)

func TestPlansRequireDistinctTargetsAndDoNotReuseIdentifiers(t *testing.T) {
	f := newFixture(t)
	duplicate := f.spec
	duplicate.Stages = append(duplicate.Stages, duplicate.Stages[0])
	if _, err := f.m.Create(duplicate); !errors.Is(err, registry.ErrInvalid) {
		t.Fatal("repeated stages or targets were accepted")
	}
	_, err := f.m.Create(f.spec)
	require(t, err)
	changed := f.spec
	changed.ObserveSeconds++
	if _, err := f.m.Create(changed); !errors.Is(err, registry.ErrConflict) {
		t.Fatal("existing plan ID was reused with a different intent")
	}
	changed.ID = "another-release"
	if _, err := f.m.Create(changed); !errors.Is(err, registry.ErrConflict) {
		t.Fatal("two active rollouts claimed the same nodes")
	}
}

func TestCandidateChangesAndForeignManifestsNeverReachPublisher(t *testing.T) {
	f := newFixture(t)
	if _, err := f.m.Stage("canary", manifestBytes(t, "second")); !errors.Is(err, registry.ErrInvalid) {
		t.Fatal("candidate for another node was accepted")
	}
	_, err := f.m.Create(f.spec)
	require(t, err)
	hash := f.spec.Stages[0].Targets[0].CandidateSHA256
	require(t, os.WriteFile(filepath.Join(f.root, "candidates", hash+".json"), []byte("changed"), 0600))
	if value := f.tick(t); value.Phase != "failed" || value.Failure != "candidate_invalid" || len(f.b.calls) != 0 {
		t.Fatal("changed candidate reached the publisher")
	}
}

func TestJournalRejectsConcurrentOwnersAndCorruptionWithoutRepair(t *testing.T) {
	f := newFixture(t)
	if second, err := Open(f.root, f.b); err == nil {
		second.Close()
		t.Fatal("two coordinators own the same journal")
	}
	_, err := f.m.Create(f.spec)
	require(t, err)
	path := filepath.Join(f.root, "plans", f.spec.ID+".json")
	require(t, os.WriteFile(path, []byte("corrupt"), 0600))
	if err := f.m.Tick(t.Context()); err == nil || len(f.b.calls) != 0 {
		t.Fatal("corrupt intent was repaired or published")
	}
	raw, err := os.ReadFile(path)
	require(t, err)
	if string(raw) != "corrupt" {
		t.Fatal("corrupt record was replaced")
	}
}

func TestPrivateJournalAndUnenrolledNodesAreNotSilentlyFixed(t *testing.T) {
	f := newFixture(t)
	node := f.b.nodes["second"]
	node.Enrolled = false
	f.b.nodes["second"] = node
	if _, err := f.m.Create(f.spec); !errors.Is(err, registry.ErrConflict) {
		t.Fatal("unenrolled node entered an automatic rollout")
	}
	require(t, f.m.Close())
	require(t, os.Remove(filepath.Join(f.root, "format.json")))
	if _, err := Open(f.root, f.b); err == nil {
		t.Fatal("lost journal format was recreated over existing candidates")
	}
	if _, err := os.Stat(filepath.Join(f.root, "format.json")); !os.IsNotExist(err) {
		t.Fatal("missing metadata was repaired")
	}
	require(t, safeio.CheckAccess(f.root, os.ModeDir|0700, os.Geteuid(), -1))
}
