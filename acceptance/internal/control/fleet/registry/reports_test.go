package registry

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
)

func TestStatusRejectsStaleConflictingAndForeignReports(t *testing.T) {
	f := newFixture(t)
	_, token := f.enroll(t, "entry")
	desired, err := f.store.Publish("entry", publication(t, "entry", 0))
	require(t, err)
	report := readyReport(f, "entry", desired, 2)
	require(t, f.store.UpdateStatus("entry", token, report))
	require(t, f.store.UpdateStatus("entry", token, report))
	stale := report
	stale.Attempt--
	if err := f.store.UpdateStatus("entry", token, stale); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale health report accepted: %v", err)
	}
	conflicting := report
	conflicting.Phase = "failed"
	if err := f.store.UpdateStatus("entry", token, conflicting); !errors.Is(err, ErrConflict) {
		t.Fatalf("same attempt changed its result: %v", err)
	}
	foreign := report
	foreign.NodeID = "exit"
	if err := f.store.UpdateStatus("entry", token, foreign); !errors.Is(err, ErrInvalid) {
		t.Fatalf("foreign node status accepted: %v", err)
	}
	f.reopen(t)
	view, err := f.store.Node("entry")
	require(t, err)
	if view.Status == nil || *view.Status != report || view.LastSeen != f.now.Unix() {
		t.Fatalf("rejected status changed stored evidence: %+v", view)
	}
}

func TestRegistryListsPublishedRecordsButIgnoresInterruptedStagingFile(t *testing.T) {
	f := newFixture(t)
	f.enroll(t, "entry")
	f.enroll(t, "exit")
	require(t, os.WriteFile(filepath.Join(f.root, "nodes", ".nhop-interrupted"), []byte(`partial`), 0600))
	nodes, err := f.store.List()
	require(t, err)
	if len(nodes) != 2 || nodes[0].NodeID != "entry" || nodes[1].NodeID != "exit" {
		t.Fatalf("incorrect registry inventory: %+v", nodes)
	}
}

func TestConcurrentPublishersCannotBothWinTheSameRevision(t *testing.T) {
	f := newFixture(t)
	f.enroll(t, "entry")
	requests := []wire.Publish{publication(t, "entry", 0), publication(t, "entry", 0)}
	results := make(chan error, 2)
	var workers sync.WaitGroup
	for _, request := range requests {
		workers.Go(func() { _, err := f.store.Publish("entry", request); results <- err })
	}
	workers.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("publication CAS failed: success=%d conflicts=%d", successes, conflicts)
	}
}

func TestReplayedHeartbeatCannotRefreshTheCompletedAttemptTime(t *testing.T) {
	f := newFixture(t)
	_, token := f.enroll(t, "entry")
	desired, err := f.store.Publish("entry", publication(t, "entry", 0))
	require(t, err)
	report := readyReport(f, "entry", desired, 1)
	require(t, f.store.UpdateStatus("entry", token, report))
	first, err := f.store.Node("entry")
	require(t, err)
	f.now = f.now.Add(time.Minute)
	require(t, f.store.UpdateStatus("entry", token, report))
	f.reopen(t)
	replayed, err := f.store.Node("entry")
	require(t, err)
	if replayed.LastSeen != f.now.Unix() || replayed.StatusReceivedAt != first.StatusReceivedAt {
		t.Fatal("duplicate heartbeat advanced the completed-attempt timestamp")
	}
	report.Attempt++
	require(t, f.store.UpdateStatus("entry", token, report))
	advanced, err := f.store.Node("entry")
	require(t, err)
	if advanced.StatusReceivedAt != f.now.Unix() || advanced.StatusReceivedAt <= replayed.StatusReceivedAt {
		t.Fatal("new attempt did not receive its own controller timestamp")
	}
}
