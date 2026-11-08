package paneltest

import (
	"encoding/json"
	"testing"

	"vpn-script/internal/node/fleet/pull"
)

func TestPanelEnrollsDownloadsBootstrapStagesCandidateAndRevokes(t *testing.T) {
	f := newFixture(t)
	f.login(t)
	response, body := f.send(t, f.form(t, "/panel/enroll", map[string]string{"node": "node-a", "group": "canary", "validity": "600"}, nil))
	var bootstrap pull.Bootstrap
	require(t, json.Unmarshal([]byte(body), &bootstrap))
	if response.StatusCode != 200 || response.Header.Get("Content-Disposition") != `attachment; filename="bootstrap-node-a.json"` || bootstrap.Endpoint != f.endpoint.URL || bootstrap.Invitation.NodeID != "node-a" {
		t.Fatal("enrollment did not return the agent bootstrap contract")
	}
	response, _ = f.send(t, f.form(t, "/panel/candidates", nil, map[string][]byte{"manifest": publication(t, "node-a").Manifest}))
	if response.StatusCode != 200 {
		t.Fatal("candidate staging failed")
	}
	assertRevision(t, f, 0)
	for range 2 {
		response, _ = f.send(t, f.form(t, "/panel/nodes/node-a/revoke", map[string]string{"confirm": "revoke"}, nil))
		if response.StatusCode != 303 {
			t.Fatal("revocation was not idempotent")
		}
	}
	value, err := f.store.Node("node-a")
	require(t, err)
	if !value.Revoked {
		t.Fatal("revocation was not persisted")
	}
}

func TestRolloutPreviewClaimsNothingAndConfirmationPersistsPlan(t *testing.T) {
	f := newFixture(t)
	f.login(t)
	spec := f.plan(t)
	path := f.stage(t, "rollout", spec)
	response, _ := f.get(t, path)
	if response.StatusCode != 200 {
		t.Fatal("rollout review failed")
	}
	values, err := f.rollouts.List()
	require(t, err)
	if len(values) != 0 {
		t.Fatal("preview persisted a rollout")
	}
	assertRevision(t, f, 0)
	response, _ = f.send(t, f.form(t, path, map[string]string{"confirm": "apply"}, nil))
	if response.StatusCode != 303 {
		t.Fatal("rollout confirmation failed")
	}
	value, err := f.rollouts.Get(spec.ID)
	require(t, err)
	if value.Phase != "running" || value.TotalNodes != 2 || value.PublishedNodes != 0 {
		t.Fatal("confirmation did not persist the exact prepared plan")
	}
	response, _ = f.send(t, f.form(t, "/panel/rollouts/"+spec.ID+"/cancel", map[string]string{"confirm": "cancel"}, nil))
	if response.StatusCode != 303 {
		t.Fatal("rollout cancellation failed")
	}
	value, err = f.rollouts.Get(spec.ID)
	require(t, err)
	if value.Phase != "canceled" {
		t.Fatal("rollout cancellation was not persisted")
	}
}
