package rollout

import (
	"testing"
	"time"

	"vpn-script/internal/platform/safeio"
)

func TestCommittedAssignmentIsRecoveredAfterLostResultAndRenewal(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(f.spec)
	require(t, err)
	f.b.ambiguous = true
	if err := f.m.Tick(t.Context()); err == nil {
		t.Fatal("fixture did not interrupt after committing the assignment")
	}
	before, err := f.m.Get(f.spec.ID)
	require(t, err)
	if before.PublishedNodes != 0 || len(f.b.calls) != 1 {
		t.Fatal("fixture did not leave durable publication intent")
	}
	node := f.b.nodes["canary"]
	node.Desired.Revision++
	node.Desired.RequestID = safeio.Hash([]byte("lease renewal"))
	f.b.nodes["canary"] = node
	f.reopen(t)
	value := f.tick(t)
	if value.PublishedNodes != 1 || value.Nodes[0].PublishedRevision != 2 || len(f.b.calls) != 1 {
		t.Fatal("recovery overwrote, duplicated or rejected the already committed assignment")
	}
	f.passStage(t, "canary")
	if value := f.tick(t); value.Stage != 1 || value.PublishedNodes != 2 {
		t.Fatal("recovered publication cannot continue through the normal gate")
	}
}

func TestControllerObservationGapRestartsSoakInsteadOfCountingDowntime(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(f.spec)
	require(t, err)
	f.tick(t)
	f.ready("canary")
	before := f.tick(t)
	f.now = f.now.Add(40 * time.Second)
	f.ready("canary")
	f.reopen(t)
	after := f.tick(t)
	if after.Stage != 0 || after.Nodes[0].HealthySince <= before.Nodes[0].HealthySince {
		t.Fatal("controller downtime was counted as a healthy soak")
	}
	f.now = f.now.Add(10 * time.Second)
	f.ready("canary")
	if result := f.tick(t); result.Stage != 1 {
		t.Fatal("fresh observation window could not recover after restart")
	}
}

func TestExternalPublicationAndRevocationStopTheRollout(t *testing.T) {
	for _, change := range []string{"revoked", "other-publication", "wrong-generation"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			_, err := f.m.Create(f.spec)
			require(t, err)
			f.tick(t)
			f.ready("canary")
			node := f.b.nodes["canary"]
			switch change {
			case "revoked":
				node.Revoked = true
			case "other-publication":
				node.Desired.PublicationID = safeio.Hash([]byte("another publisher"))
			case "wrong-generation":
				node.Status.Generation = safeio.Hash([]byte("unexpected installed config"))
			}
			f.b.nodes["canary"] = node
			if value := f.tick(t); value.Phase != "failed" || value.FailedNode != "canary" || len(f.b.calls) != 1 {
				t.Fatal("unsafe node state did not stop publication")
			}
		})
	}
}
