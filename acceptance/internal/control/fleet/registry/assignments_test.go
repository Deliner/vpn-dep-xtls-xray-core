package registry

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	contract "vpn-script/internal/core/fleet"
)

func TestPublicationCASIdempotenceAndSignedNodeDelivery(t *testing.T) {
	f := newFixture(t)
	invite, token := f.enroll(t, "entry")
	_, otherToken := f.enroll(t, "exit")
	request := publication(t, "entry", 0)
	desired, err := f.store.Publish("entry", request)
	require(t, err)
	repeated, err := f.store.Publish("entry", request)
	require(t, err)
	if repeated != desired {
		t.Fatal("lost response retry created a second revision")
	}
	if _, err := f.store.Publish("entry", publication(t, "entry", 0)); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale publisher overwrote desired state: %v", err)
	}
	if _, err := f.store.Assignment("entry", otherToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("another node read secret manifest: %v", err)
	}
	f.reopen(t)
	raw, err := f.store.Assignment("entry", token)
	require(t, err)
	value, receipt, err := contract.Accept(raw, contract.Trust{FleetID: invite.FleetID, NodeID: invite.NodeID, PublicKey: invite.PublicKey}, contract.Receipt{}, f.now)
	require(t, err)
	if !bytes.Equal(value.Manifest, request.Manifest) || receipt.PayloadSHA256 != desired.PayloadSHA256 || receipt.Revision != 1 {
		t.Fatal("served payload is not the published node assignment")
	}
}

func TestPublicationRequestIDCannotBeReusedWithDifferentContent(t *testing.T) {
	f := newFixture(t)
	f.enroll(t, "entry")
	request := publication(t, "entry", 0)
	_, err := f.store.Publish("entry", request)
	require(t, err)
	request.Manifest = append(request.Manifest, '\n')
	if _, err := f.store.Publish("entry", request); !errors.Is(err, ErrConflict) {
		t.Fatalf("same request ID silently changed payload: %v", err)
	}
}

func TestPullRenewsExpiredAssignmentWithHigherRevision(t *testing.T) {
	f := newFixture(t)
	invite, token := f.enroll(t, "entry")
	request := publication(t, "entry", 0)
	before, err := f.store.Publish("entry", request)
	require(t, err)
	f.now = f.now.Add(2 * time.Hour)
	raw, err := f.store.Assignment("entry", token)
	require(t, err)
	value, receipt, err := contract.Accept(raw, contract.Trust{FleetID: invite.FleetID, NodeID: invite.NodeID, PublicKey: invite.PublicKey}, contract.Receipt{Revision: before.Revision, PayloadSHA256: before.PayloadSHA256}, f.now)
	require(t, err)
	if receipt.Revision != 2 || value.ExpiresAt != f.now.Unix()+request.ValiditySeconds || !bytes.Equal(value.Manifest, request.Manifest) {
		t.Fatal("expired lease was not renewed without changing manifest")
	}
	repeated, err := f.store.Assignment("entry", token)
	require(t, err)
	if !bytes.Equal(repeated, raw) {
		t.Fatal("every poll creates a new assignment")
	}
	node, err := f.store.Node("entry")
	require(t, err)
	if node.Desired.OriginRequestID() != request.RequestID || node.Desired.RequestID == request.RequestID {
		t.Fatal("renewal lost the original publication identity")
	}
}

func TestDamagedAssignmentCannotBeDeliveredOrRenewed(t *testing.T) {
	f := newFixture(t)
	_, token := f.enroll(t, "entry")
	desired, err := f.store.Publish("entry", publication(t, "entry", 0))
	require(t, err)
	path := filepath.Join(f.root, "assignments", "entry", desired.EnvelopeSHA256+".json")
	require(t, os.WriteFile(path, []byte(`{}`), 0600))
	for _, elapsed := range []time.Duration{0, 2 * time.Hour} {
		f.now = f.now.Add(elapsed)
		if raw, err := f.store.Assignment("entry", token); err == nil || len(raw) != 0 {
			t.Fatalf("damaged immutable artifact delivered: err=%v", err)
		}
	}
}
