package scale

import (
	"encoding/pem"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/server"
	"vpn-script/internal/platform/fleetapi"
)

const fleetID = "capacity-fixture"

type fixture struct {
	root   string
	store  *registry.Store
	server *httptest.Server
	client *fleetapi.Client
	admin  string
	tokens sync.Map
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
	f := &fixture{root: filepath.Join(base, "registry")}
	require(t, registry.Initialize(f.root, fleetID))
	f.store, err = registry.Open(f.root)
	require(t, err)
	t.Cleanup(func() { require(t, f.store.Close()) })
	f.startAPI(t)
	f.admin, err = registry.ReadAdminCredential(filepath.Join(f.root, "admin-credential.json"))
	require(t, err)
	return f
}

func (f *fixture) startAPI(t *testing.T) {
	t.Helper()
	f.server = httptest.NewTLSServer(server.New(f.store, nil))
	t.Cleanup(f.server.Close)
	ca := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.server.Certificate().Raw})
	var err error
	f.client, err = fleetapi.New(f.server.URL, ca)
	require(t, err)
	t.Cleanup(f.client.Close)
}
