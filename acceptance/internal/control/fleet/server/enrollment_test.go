package server_test

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"vpn-script/internal/control/fleet/server"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/node/fleet/pull"
)

func TestLostEnrollmentReplyAcrossRealRegistryAndAgentRestart(t *testing.T) {
	f := newFixture(t)
	root, runner, requests := runnerWithLostReply(t, f)
	if _, err := runner.Step(context.Background()); err == nil {
		t.Fatal("lost successful reply reported success")
	}
	node, err := f.store.Node("node-a")
	require(t, err)
	if !node.Enrolled {
		t.Fatal("fixture failed to persist server-side enrollment before losing reply")
	}
	require(t, runner.Close())
	restarted, err := pull.Open(root)
	require(t, err)
	defer restarted.Close()
	status, err := restarted.Step(context.Background())
	require(t, err)
	_, err = restarted.Step(context.Background())
	require(t, err)
	node, err = f.store.Node("node-a")
	require(t, err)
	if requests.Load() != 2 || status.Phase != "waiting" || node.Status == nil || *node.Status != status || node.LastSeen == 0 {
		t.Fatal("durable enrollment retry or subsequent heartbeat failed")
	}
}

func runnerWithLostReply(t *testing.T, f *fixture) (string, *pull.Runner, *atomic.Int32) {
	t.Helper()
	api := server.New(f.store, nil)
	requests := new(atomic.Int32)
	endpoint := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/enroll" && requests.Add(1) == 1 {
			recorded := httptest.NewRecorder()
			api.ServeHTTP(recorded, r)
			w.WriteHeader(http.StatusInternalServerError)
			return // Simulate losing success after the real registry committed it.
		}
		api.ServeHTTP(w, r)
	}))
	t.Cleanup(endpoint.Close)
	invite, err := f.store.Invite(wire.Invite{NodeID: "node-a", Group: "main", ValiditySeconds: 600})
	require(t, err)
	bootstrap := pull.Bootstrap{Version: 1, Endpoint: endpoint.URL, Invitation: invite,
		CAPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: endpoint.Certificate().Raw})}
	root := filepath.Join(t.TempDir(), "agent")
	require(t, pull.Initialize(root, bootstrap))
	runner, err := pull.Open(root)
	require(t, err)
	t.Cleanup(func() { require(t, runner.Close()) })
	return root, runner, requests
}
