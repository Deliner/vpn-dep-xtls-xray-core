package server_test

import (
	"bytes"
	"context"
	"encoding/pem"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/control/fleet/server"
	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/fleetapi"
)

func TestHTTPSPublicationPullAndReplay(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	invite, token := f.enroll(t, "node-a")
	raw, err := f.client.Assignment(ctx, "node-a", token)
	require(t, err)
	if len(raw) != 0 {
		t.Fatal("unconfigured node received an assignment")
	}
	request := publication(t, "node-a")
	desired, err := f.client.Publish(ctx, "node-a", f.admin, request)
	require(t, err)
	retried, err := f.client.Publish(ctx, "node-a", f.admin, request)
	require(t, err)
	if retried != desired {
		t.Fatal("lost-response retry created another revision")
	}
	raw, err = f.client.Assignment(ctx, "node-a", token)
	require(t, err)
	trust := contract.Trust{FleetID: invite.FleetID, NodeID: invite.NodeID, PublicKey: invite.PublicKey}
	value, receipt, err := contract.Accept(raw, trust, contract.Receipt{}, time.Now())
	require(t, err)
	if !bytes.Equal(value.Manifest, request.Manifest) || receipt.Revision != desired.Revision || contract.Digest(raw) != desired.EnvelopeSHA256 {
		t.Fatal("HTTPS altered the signed assignment")
	}
	request.RequestID = contract.Digest([]byte("stale publisher"))
	_, err = f.client.Publish(ctx, "node-a", f.admin, request)
	requireHTTP(t, err, http.StatusConflict)
}

func TestProductionWorkerAdvancesOnlyAfterFreshCanaryReports(t *testing.T) {
	f := newFixture(t)
	spec, tokens := f.rolloutPlan(t)
	client := f.production(t)
	created, err := client.CreateRollout(t.Context(), f.admin, spec)
	require(t, err)
	repeated, err := client.CreateRollout(t.Context(), f.admin, spec)
	require(t, err)
	if created.SpecSHA256 != repeated.SpecSHA256 {
		t.Fatal("HTTP retry changed rollout intent")
	}
	for index, name := range []string{"node-a", "node-b"} {
		waitFor(t, func() bool {
			node, err := client.Node(t.Context(), name, f.admin)
			return err == nil && node.Desired.Revision == 1
		})
		if index == 0 {
			next, err := client.Assignment(t.Context(), "node-b", tokens["node-b"])
			require(t, err)
			if len(next) != 0 {
				t.Fatal("non-canary assignment was released early")
			}
		}
		f.readyHTTP(t, client, name, tokens[name], 1)
		waitFor(t, func() bool {
			value, err := client.Rollout(t.Context(), spec.ID, f.admin)
			return err == nil && value.Nodes[index].Phase == "observing"
		})
		time.Sleep(1100 * time.Millisecond)
		f.readyHTTP(t, client, name, tokens[name], 2)
	}
	waitFor(t, func() bool {
		value, err := client.Rollout(t.Context(), spec.ID, f.admin)
		return err == nil && value.Phase == "succeeded" && value.ReadyNodes == 2
	})
}

func (f *fixture) production(t *testing.T) *fleetapi.Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.New(f.store, f.rollouts).Serve(ctx, listener, f.server.TLS.Certificates[0]) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require(t, err)
		case <-time.After(5 * time.Second):
			t.Error("production controller did not join its rollout worker")
		}
	})
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.server.Certificate().Raw})
	client, err := fleetapi.New("https://"+listener.Addr().String(), ca)
	require(t, err)
	t.Cleanup(client.Close)
	return client
}

func waitFor(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("rollout did not reach the expected state")
}

func (f *fixture) readyHTTP(t *testing.T, client *fleetapi.Client, id, token string, attempt uint64) {
	t.Helper()
	node, err := client.Node(t.Context(), id, f.admin)
	require(t, err)
	raw, err := client.Assignment(t.Context(), id, token)
	require(t, err)
	if contract.Digest(raw) != node.Desired.EnvelopeSHA256 {
		t.Fatal("rollout did not deliver its exact signed envelope")
	}
	report := wire.Status{FleetID: node.FleetID, NodeID: id, Phase: "ready", Attempt: attempt,
		AcceptedRevision: node.Desired.Revision, AppliedRevision: node.Desired.Revision,
		Generation: node.Desired.Generation, AttemptedAt: time.Now().Unix()}
	require(t, client.Report(t.Context(), token, report))
}

func TestProductionControllerStopsOnUnreadableRolloutIntent(t *testing.T) {
	f := newFixture(t)
	spec, _ := f.rolloutPlan(t)
	_, err := f.client.CreateRollout(t.Context(), f.admin, spec)
	require(t, err)
	require(t, os.WriteFile(filepath.Join(f.root, "rollouts/plans", spec.ID+".json"), []byte("corrupt"), 0600))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.New(f.store, f.rollouts).Serve(ctx, listener, f.server.TLS.Certificates[0]) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("controller ignored its failed durable coordinator")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("controller did not stop after a journal failure")
	}
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err == nil {
		connection.Close()
		t.Fatal("failed controller left its listener open")
	}
}

func TestProductionTLSListenerShutsDownAndReleasesSocket(t *testing.T) {
	f := newFixture(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.New(f.store, nil).Serve(ctx, listener, f.server.TLS.Certificates[0]) }()
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.server.Certificate().Raw})
	client, err := fleetapi.New("https://"+listener.Addr().String(), ca)
	require(t, err)
	defer client.Close()
	_, err = client.List(context.Background(), f.admin)
	require(t, err)
	cancel()
	select {
	case err := <-done:
		require(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("TLS listener did not stop")
	}
	connection, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err == nil {
		connection.Close()
		t.Fatal("shutdown left a listening socket")
	}
}

func TestHTTPSStatusAndRevocation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	_, token := f.enroll(t, "node-a")
	desired, err := f.client.Publish(ctx, "node-a", f.admin, publication(t, "node-a"))
	require(t, err)
	status := wire.Status{FleetID: "test-fleet", NodeID: "node-a", Phase: "ready", Attempt: 1,
		AcceptedRevision: desired.Revision, AppliedRevision: desired.Revision, Generation: desired.Generation, AttemptedAt: time.Now().Unix()}
	require(t, f.client.Report(ctx, token, status))
	value, err := f.client.Node(ctx, "node-a", f.admin)
	require(t, err)
	if value.Status == nil || *value.Status != status || value.LastSeen == 0 || value.Group != "canary" {
		t.Fatal("administrator cannot observe the reported node state")
	}
	values, err := f.client.List(ctx, f.admin)
	require(t, err)
	if len(values) != 1 || values[0].NodeID != "node-a" {
		t.Fatal("node list mismatch")
	}
	require(t, f.client.Revoke(ctx, "node-a", f.admin))
	_, err = f.client.Assignment(ctx, "node-a", token)
	requireHTTP(t, err, http.StatusForbidden)
	requireHTTP(t, f.client.Report(ctx, token, status), http.StatusForbidden)
	value, err = f.client.Node(ctx, "node-a", f.admin)
	require(t, err)
	if !value.Revoked || value.Status == nil || *value.Status != status {
		t.Fatal("revocation lost the last reported state")
	}
}
