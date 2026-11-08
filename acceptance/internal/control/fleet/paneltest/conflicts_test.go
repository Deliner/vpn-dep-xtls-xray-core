package paneltest

import (
	"net/http/cookiejar"
	"testing"
)

func TestRolloutConfirmationRechecksNodeClaims(t *testing.T) {
	f := newFixture(t)
	f.login(t)
	spec := f.plan(t)
	path := f.stage(t, "rollout", spec)
	other := spec
	other.ID = "competing-rollout"
	_, err := f.rollouts.Create(other)
	require(t, err)
	response, _ := f.send(t, f.form(t, path, map[string]string{"confirm": "apply"}, nil))
	if response.StatusCode != 409 {
		t.Fatal("review confirmation stole nodes from another rollout")
	}
	values, err := f.rollouts.List()
	require(t, err)
	if len(values) != 1 || values[0].ID != other.ID {
		t.Fatal("rejected confirmation persisted another plan")
	}
	assertRevision(t, f, 0)
}

func TestReviewCannotBeConfirmedFromAnotherBrowserSession(t *testing.T) {
	f := newFixture(t)
	f.enroll(t, "node-a")
	f.login(t)
	path := f.stage(t, "publication", publication(t, "node-a"))
	var err error
	f.client.Jar, err = cookiejar.New(nil)
	require(t, err)
	f.login(t)
	response, _ := f.send(t, f.form(t, path, map[string]string{"confirm": "apply"}, nil))
	if response.StatusCode != 410 {
		t.Fatal("another browser session confirmed a private review")
	}
	assertRevision(t, f, 0)
}
