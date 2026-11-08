package registry

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/safeio"
)

func TestEnrollmentConsumesInvitationAndRecoversLostResponse(t *testing.T) {
	f := newFixture(t)
	invite, token := f.enroll(t, "entry")
	f.reopen(t)
	f.now = f.now.Add(time.Hour)
	identity, err := f.store.Enroll(wire.Enrollment{NodeID: "entry", Token: invite.Token, Credential: token})
	require(t, err)
	if identity.FleetID != invite.FleetID || identity.NodeID != "entry" {
		t.Fatalf("wrong enrollment identity: %+v", identity)
	}
	other, err := credential.NewToken()
	require(t, err)
	if _, err := f.store.Enroll(wire.Enrollment{NodeID: "entry", Token: invite.Token, Credential: other}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("used invitation replaced node credential: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(f.root, "nodes", "entry.json"))
	require(t, err)
	if strings.Contains(string(raw), token) || strings.Contains(string(raw), invite.Token) {
		t.Fatal("registry stored plaintext node credentials")
	}
}

func TestExpiredAndCrossNodeInvitationsAreRejected(t *testing.T) {
	f := newFixture(t)
	a, err := f.store.Invite(wire.Invite{NodeID: "entry", Group: "main", ValiditySeconds: 10})
	require(t, err)
	_, err = f.store.Invite(wire.Invite{NodeID: "exit", Group: "main", ValiditySeconds: 10})
	require(t, err)
	token, err := credential.NewToken()
	require(t, err)
	for _, node := range []string{"exit", "../entry", "unknown"} {
		if _, err := f.store.Enroll(wire.Enrollment{NodeID: node, Token: a.Token, Credential: token}); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("foreign invitation accepted for %s: %v", node, err)
		}
	}
	f.now = f.now.Add(10 * time.Second)
	if _, err := f.store.Enroll(wire.Enrollment{NodeID: "entry", Token: a.Token, Credential: token}); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expired invitation accepted: %v", err)
	}
}

func TestRevocationSurvivesRestartAndDeniesEveryNodeOperation(t *testing.T) {
	f := newFixture(t)
	invite, token := f.enroll(t, "entry")
	require(t, f.store.Revoke("entry"))
	f.reopen(t)
	if _, err := f.store.Assignment("entry", token); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked assignment access: %v", err)
	}
	if err := f.store.UpdateStatus("entry", token, wire.Status{}); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked status access: %v", err)
	}
	if _, err := f.store.Enroll(wire.Enrollment{NodeID: "entry", Token: invite.Token, Credential: token}); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked enrollment restored access: %v", err)
	}
	if _, err := f.store.Publish("entry", publication(t, "entry", 0)); !errors.Is(err, ErrRevoked) {
		t.Fatalf("published to revoked node: %v", err)
	}
	if _, err := f.store.Invite(wire.Invite{NodeID: "entry", Group: "main", ValiditySeconds: 600}); !errors.Is(err, ErrConflict) {
		t.Fatalf("revoked identity was reused: %v", err)
	}
}

func TestRegistryOwnershipLockAndAdministratorCredential(t *testing.T) {
	f := newFixture(t)
	admin, err := ReadAdminCredential(filepath.Join(f.root, "admin-credential.json"))
	require(t, err)
	_, nodeToken := f.enroll(t, "entry")
	if !f.store.AuthenticateAdmin(admin) || f.store.AuthenticateAdmin(nodeToken) || f.store.AuthenticateAdmin("invalid") {
		t.Fatal("administrator credential boundary failed")
	}
	if second, err := Open(f.root); err == nil {
		second.Close()
		t.Fatal("two controllers acquired the same registry")
	}
	require(t, f.store.Close())
	if f.store.AuthenticateAdmin(admin) {
		t.Fatal("closed controller accepted administrator credential")
	}
	if _, err := f.store.List(); !errors.Is(err, ErrClosed) {
		t.Fatalf("closed controller retained storage access: %v", err)
	}
	require(t, os.Chmod(filepath.Join(f.root, "authority.json"), 0644))
	if store, err := Open(f.root); err == nil {
		store.Close()
		t.Fatal("public authority file accepted")
	}
	require(t, safeio.CheckAccess(filepath.Join(f.root, "authority.json"), 0644, os.Geteuid(), -1))
}
