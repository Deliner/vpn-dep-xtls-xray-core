package pull

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/fleetapi"
)

func TestInvalidSignedResponseCannotAdvanceRealAgentJournal(t *testing.T) {
	f := newFixture(t)
	f.assignment = []byte(`{"tampered":true}`)
	before, err := f.runner.agent.Status()
	require(t, err)
	status, err := f.runner.Step(context.Background())
	if err == nil || status != before {
		t.Fatal("invalid signed response advanced the agent")
	}
	after, err := f.runner.agent.Status()
	require(t, err)
	if after != before {
		t.Fatal("invalid signature changed the durable journal")
	}
}

func TestPullReportsInstallerFailureWithoutSendingRawError(t *testing.T) {
	f := newFixture(t)
	f.assignment = []byte(`{"signed":"opaque fixture"}`)
	backend := &rejectedInstaller{status: wire.Status{FleetID: "test-fleet", NodeID: "node-a", Phase: "failed", Attempt: 1, AcceptedRevision: 1, AttemptedAt: time.Now().Unix()}}
	f.runner.agent = backend
	status, err := f.runner.Step(context.Background())
	if err == nil || backend.calls != 1 || !bytes.Equal(backend.raw, f.assignment) || status != backend.status {
		t.Fatal("failed installer result lost at pull boundary")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.reports) != 1 || f.reports[0] != backend.status {
		t.Fatal("failed application was not reported")
	}
}

func TestRevocationTerminatesPollingWithoutCallingInstaller(t *testing.T) {
	f := newFixture(t)
	f.revoked = true
	backend := &rejectedInstaller{}
	f.runner.agent = backend
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := f.runner.Run(ctx, time.Second, nil)
	var response *fleetapi.HTTPError
	if !errors.As(err, &response) || response.Status != http.StatusForbidden || backend.calls != 0 {
		t.Fatalf("revoked pull did not stop safely: %v", err)
	}
}

func TestPollingRecoversAfterControllerOutageAndHonorsCancellation(t *testing.T) {
	f := newFixture(t)
	f.failFetch = true
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	observed := 0
	err := f.runner.Run(ctx, time.Second, func(status wire.Status, err error) {
		observed++
		if observed == 1 && err == nil {
			t.Error("controller outage reported success")
		}
		f.mu.Lock()
		f.failFetch = false
		f.mu.Unlock()
		if err == nil && status.Phase == "waiting" {
			cancel()
		}
	})
	if !errors.Is(err, context.Canceled) || observed != 2 {
		t.Fatalf("poll recovery/cancellation failed: %v, attempts %d", err, observed)
	}
}
