package rollout

import (
	"context"
	"errors"
	"testing"
	"time"

	"vpn-script/internal/control/fleet/registry"
)

func TestCanaryRequiresFreshDistinctReportsBeforeLaterStages(t *testing.T) {
	f := newFixture(t)
	first, err := f.m.Create(f.spec)
	require(t, err)
	repeated, err := f.m.Create(f.spec)
	require(t, err)
	if first.SpecSHA256 != repeated.SpecSHA256 || len(f.b.calls) != 0 {
		t.Fatal("creating or retrying a plan published a node")
	}
	f.tick(t)
	if len(f.b.calls) != 1 || f.b.calls[0] != "canary" {
		t.Fatal("later nodes were published before the canary gate")
	}
	f.ready("canary")
	f.tick(t)
	f.now = f.now.Add(10 * time.Second)
	node := f.b.nodes["canary"]
	node.LastSeen = f.now.Unix() // Exact replay cannot supply the second report.
	f.b.nodes["canary"] = node
	if value := f.tick(t); value.Stage != 0 || value.ReadyNodes != 0 {
		t.Fatal("elapsed time and a replay promoted the canary")
	}
	f.ready("canary")
	if value := f.tick(t); value.Stage != 1 || len(f.b.calls) != 1 {
		t.Fatal("healthy canary did not durably advance before publication")
	}
	f.passStage(t, "second")
	result := f.passStage(t, "third")
	if result.Phase != "succeeded" || result.ReadyNodes != 3 || result.PublishedNodes != 3 || len(f.b.calls) != 3 {
		t.Fatal("ordered rollout did not complete every stage exactly once")
	}
}

func TestFailureInPreviouslyPassedCanaryStopsLaterPublication(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(f.spec)
	require(t, err)
	f.passStage(t, "canary")
	node := f.b.nodes["canary"]
	node.Status.Phase = "failed"
	f.b.nodes["canary"] = node
	value := f.tick(t)
	if value.Phase != "failed" || value.Failure != "install_failed" || value.FailedNode != "canary" || len(f.b.calls) != 1 {
		t.Fatal("failed canary did not block the remaining fleet")
	}
	f.ready("canary")
	if value := f.tick(t); value.Phase != "failed" || len(f.b.calls) != 1 {
		t.Fatal("failure was silently reset")
	}
}

func TestStaleReportsAndMissingAgentsCannotPassTheDeadline(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(f.spec)
	require(t, err)
	f.tick(t)
	f.ready("canary")
	f.tick(t)
	f.now = f.now.Add(31 * time.Second)
	if value := f.tick(t); value.Nodes[0].HealthySince != 0 || value.Stage != 0 {
		t.Fatal("stale canary retained its observation window")
	}
	f.now = f.now.Add(time.Minute)
	value := f.tick(t)
	if value.Phase != "failed" || value.Failure != "stage_timeout" || len(f.b.calls) != 1 {
		t.Fatal("missing agent caused publication beyond the deadline")
	}
}

func TestCancellationIsDurableAndDoesNotReplaceInstalledAssignments(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(f.spec)
	require(t, err)
	f.tick(t)
	desired := f.b.nodes["canary"].Desired
	_, err = f.m.Cancel(f.spec.ID)
	require(t, err)
	f.reopen(t)
	repeated, err := f.m.Cancel(f.spec.ID)
	require(t, err)
	f.ready("canary")
	f.now = f.now.Add(time.Minute)
	value := f.tick(t)
	if value.Phase != "canceled" || repeated.Phase != "canceled" || len(f.b.calls) != 1 || f.b.nodes["canary"].Desired != desired {
		t.Fatal("cancellation published or removed a node assignment")
	}
	require(t, f.m.Close())
	if !errors.Is(f.m.Tick(context.Background()), registry.ErrClosed) {
		t.Fatal("closed coordinator still performs work")
	}
}

func TestLateReplayOfAnEarlierSecondReportDoesNotCompleteSoak(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(f.spec)
	require(t, err)
	f.tick(t)
	f.ready("canary")
	f.tick(t)
	f.now = f.now.Add(time.Second)
	f.ready("canary")
	f.tick(t)
	f.now = f.now.Add(10 * time.Second)
	node := f.b.nodes["canary"]
	node.LastSeen = f.now.Unix()
	f.b.nodes["canary"] = node
	if value := f.tick(t); value.Stage != 0 || value.ReadyNodes != 0 {
		t.Fatal("replayed pre-window report was treated as a fresh completed attempt")
	}
	f.ready("canary")
	if value := f.tick(t); value.Stage != 1 {
		t.Fatal("a new post-window report did not pass the gate")
	}
}
