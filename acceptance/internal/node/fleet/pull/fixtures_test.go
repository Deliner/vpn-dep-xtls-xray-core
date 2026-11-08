package pull

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/credential"
)

type fixture struct {
	mu            sync.Mutex
	runner        *Runner
	root          string
	bootstrap     Bootstrap
	enrollments   []wire.Enrollment
	reports       []wire.Status
	loseResponse  bool
	wrongIdentity bool
	revoked       bool
	failFetch     bool
	assignment    []byte
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{root: filepath.Join(t.TempDir(), "pull")}
	key, _, err := ed25519.GenerateKey(nil)
	require(t, err)
	token, err := credential.NewToken()
	require(t, err)
	server := httptest.NewTLSServer(http.HandlerFunc(f.serve))
	t.Cleanup(server.Close)
	f.bootstrap = Bootstrap{Version: 1, Endpoint: server.URL,
		CAPEM:      pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}),
		Invitation: wire.Invitation{FleetID: "test-fleet", NodeID: "node-a", PublicKey: key, Token: token, ExpiresAt: time.Now().Unix() + 600}}
	require(t, Initialize(f.root, f.bootstrap))
	f.open(t)
	return f
}

func (f *fixture) open(t *testing.T) {
	t.Helper()
	if f.runner != nil {
		require(t, f.runner.Close())
	}
	var err error
	f.runner, err = Open(f.root)
	require(t, err)
	runner := f.runner
	t.Cleanup(func() { require(t, runner.Close()) })
}

func (f *fixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/v1/enroll":
		f.enroll(w, r)
	case "/v1/nodes/node-a/assignment":
		if f.revoked {
			w.WriteHeader(http.StatusForbidden)
		} else if f.failFetch {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if len(f.assignment) == 0 {
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.Write(f.assignment)
		}
	case "/v1/nodes/node-a/status":
		var report wire.Status
		if json.NewDecoder(r.Body).Decode(&report) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.reports = append(f.reports, report)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fixture) enroll(w http.ResponseWriter, r *http.Request) {
	var request wire.Enrollment
	if json.NewDecoder(r.Body).Decode(&request) != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	f.enrollments = append(f.enrollments, request)
	if f.loseResponse {
		f.loseResponse = false
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	identity := wire.Identity{FleetID: "test-fleet", NodeID: "node-a"}
	if f.wrongIdentity {
		identity.NodeID = "foreign"
	}
	json.NewEncoder(w).Encode(identity)
}

type rejectedInstaller struct {
	status wire.Status
	raw    []byte
	calls  int
}

func (a *rejectedInstaller) Status() (wire.Status, error) { return a.status, nil }

func (a *rejectedInstaller) Apply(_ context.Context, raw []byte) (wire.Status, error) {
	a.raw, a.calls = raw, a.calls+1
	return a.status, fmt.Errorf("private installer failure")
}
