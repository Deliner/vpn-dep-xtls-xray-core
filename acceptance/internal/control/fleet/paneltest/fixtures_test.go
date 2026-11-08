package paneltest

import (
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout"
	"vpn-script/internal/control/fleet/server"
)

type fixture struct {
	store      *registry.Store
	rollouts   *rollout.Manager
	endpoint   *httptest.Server
	client     *http.Client
	credential []byte
	csrf       string
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	require(t, err)
	root := filepath.Join(base, "controller")
	require(t, registry.Initialize(root, "test-fleet"))
	store, err := registry.Open(root)
	require(t, err)
	t.Cleanup(func() { require(t, store.Close()) })
	m, err := rollout.Open(filepath.Join(root, "rollouts"), store)
	require(t, err)
	t.Cleanup(func() { require(t, m.Close()) })
	endpoint := httptest.NewTLSServer(server.New(store, m))
	t.Cleanup(endpoint.Close)
	client := endpoint.Client()
	client.Jar, err = cookiejar.New(nil)
	require(t, err)
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	raw, err := os.ReadFile(filepath.Join(root, "admin-credential.json"))
	require(t, err)
	return &fixture{store: store, rollouts: m, endpoint: endpoint, client: client, credential: raw}
}
