package paneltest

import (
	"strings"
	"testing"

	"vpn-script/internal/core/fleet/wire"
)

func TestPublicationRequiresReviewAndConfirmationAndKeepsRetryReceipt(t *testing.T) {
	f := newFixture(t)
	f.enroll(t, "node-a")
	f.login(t)
	request := publication(t, "node-a")
	path := f.stage(t, "publication", request)
	response, html := f.get(t, path)
	if response.StatusCode != 200 || !strings.Contains(html, "24443") || !strings.Contains(html, request.RequestID) {
		t.Fatal("review omitted the target publication")
	}
	assertRevision(t, f, 0)
	response, _ = f.send(t, f.form(t, path, nil, nil))
	if response.StatusCode != 400 {
		t.Fatal("publication did not require explicit confirmation")
	}
	assertRevision(t, f, 0)
	for range 2 {
		response, _ := f.send(t, f.form(t, path, map[string]string{"confirm": "apply"}, nil))
		if response.StatusCode != 303 || response.Header.Get("Location") != "/panel/nodes/node-a" {
			t.Fatal("confirmed publication or retry failed")
		}
		assertRevision(t, f, 1)
	}
}

func assertRevision(t *testing.T, f *fixture, expected uint64) {
	t.Helper()
	value, err := f.store.Node("node-a")
	require(t, err)
	if value.Desired.Revision != expected {
		t.Fatalf("unexpected desired revision: %d, expected %d", value.Desired.Revision, expected)
	}
}

func TestStaleReviewCannotOverwriteAnotherPublication(t *testing.T) {
	f := newFixture(t)
	f.enroll(t, "node-a")
	f.login(t)
	path := f.stage(t, "publication", publication(t, "node-a"))
	other := publication(t, "node-a")
	_, err := f.store.Publish("node-a", other)
	require(t, err)
	response, _ := f.send(t, f.form(t, path, map[string]string{"confirm": "apply"}, nil))
	if response.StatusCode != 409 {
		t.Fatal("stale browser review overwrote newer state")
	}
	value, err := f.store.Node("node-a")
	require(t, err)
	if value.Desired.RequestID != other.RequestID || value.Desired.Revision != 1 {
		t.Fatal("rejected review changed the live assignment")
	}
}

func TestInvalidPublicationIsRejectedBeforeReview(t *testing.T) {
	f := newFixture(t)
	f.enroll(t, "node-a")
	f.login(t)
	request := publication(t, "node-a")
	request.ExpectedRevision = 9
	response, _ := f.send(t, f.form(t, "/panel/prepare/publication", nil, map[string][]byte{"request": encoded(t, request)}))
	if response.StatusCode != 409 {
		t.Fatal("stale request reached confirmation")
	}
	response, _ = f.send(t, f.form(t, "/panel/prepare/publication", nil, map[string][]byte{"request": encoded(t, wire.Publish{})}))
	if response.StatusCode != 400 {
		t.Fatal("invalid manifest reached confirmation")
	}
	assertRevision(t, f, 0)
}
