package scale

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"vpn-script/internal/control/fleet/registry"
	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/fleetapi"
)

func (f *fixture) verify(t *testing.T, expected []wire.Node) {
	t.Helper()
	actual, err := f.client.List(t.Context(), f.admin)
	require(t, err)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("HTTPS inventory lost, duplicated, reordered or crossed node records")
	}
}

func (f *fixture) revoke(t *testing.T, expected []wire.Node) {
	t.Helper()
	for index := 0; index < len(expected); index += 10 {
		node := expected[index].NodeID
		require(t, f.client.Revoke(t.Context(), node, f.admin))
		f.requireRevoked(t, expected[index])
		// Revocation must preserve the exact signed desired state and last report.
		expected[index].Revoked = true
	}
	f.verify(t, expected)
	if len(expected) > 1 {
		f.requireActive(t, expected[1])
	}
}

func (f *fixture) requireRevoked(t *testing.T, node wire.Node) {
	t.Helper()
	token, found := f.tokens.Load(node.NodeID)
	if !found {
		t.Fatal("successful node has no authenticated credential")
	}
	_, err := f.client.Assignment(t.Context(), node.NodeID, token.(string))
	requireForbidden(t, err)
	requireForbidden(t, f.client.Report(t.Context(), token.(string), *node.Status))
}

func (f *fixture) requireActive(t *testing.T, node wire.Node) {
	t.Helper()
	token, found := f.tokens.Load(node.NodeID)
	if !found {
		t.Fatal("non-revoked node has no authenticated credential")
	}
	raw, err := f.client.Assignment(t.Context(), node.NodeID, token.(string))
	require(t, err)
	if contract.Digest(raw) != node.Desired.EnvelopeSHA256 {
		t.Fatal("revoking another node changed a non-revoked assignment")
	}
}

func requireForbidden(t *testing.T, err error) {
	t.Helper()
	var failure *fleetapi.HTTPError
	if !errors.As(err, &failure) || failure.Status != http.StatusForbidden {
		t.Fatalf("revoked credential accepted or returned the wrong error: %v", err)
	}
}

func (f *fixture) restart(t *testing.T, expected []wire.Node) {
	t.Helper()
	f.client.Close()
	f.server.Close()
	require(t, f.store.Close())
	store, err := registry.Open(f.root)
	require(t, err)
	f.store = store
	f.startAPI(t)
	f.verify(t, expected)
	for _, node := range expected {
		if node.Revoked {
			f.requireRevoked(t, node)
		} else {
			f.requireActive(t, node)
		}
	}
}
