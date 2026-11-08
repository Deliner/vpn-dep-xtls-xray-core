package paneltest

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
)

type snapshot struct {
	body, kind string
}

func panelSnapshots(t *testing.T) map[string]snapshot {
	t.Helper()
	f := newFixture(t)
	_, login := f.get(t, "/panel/login")
	f.login(t)
	previewNodes(t, f)
	spec := f.plan(t)
	rolloutReview := f.stage(t, "rollout", spec)
	publicationReview := f.stage(t, "publication", publication(t, "relay-fra-01"))
	paths := []string{"/panel/", "/panel/nodes", "/panel/nodes/entry-hel-01", "/panel/nodes/relay-fra-01",
		"/panel/rollouts", "/panel/rollouts/" + spec.ID, "/panel/enroll", "/panel/prepare", rolloutReview, publicationReview}
	// Capture the uncommitted review first; the overview below includes an active rollout.
	_, review := f.get(t, rolloutReview)
	_, err := f.rollouts.Create(spec)
	require(t, err)
	require(t, f.rollouts.Tick(t.Context()))
	values := map[string]snapshot{"/panel/login": {body: login, kind: "text/html; charset=utf-8"}}
	for _, path := range paths {
		response, html := f.get(t, path)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("snapshot %s returned %d", path, response.StatusCode)
		}
		values[path] = snapshot{body: html, kind: "text/html; charset=utf-8"}
	}
	values["/panel/example-rollout"] = snapshot{body: review, kind: "text/html; charset=utf-8"}
	values["/panel/example-publication"] = values[publicationReview]
	_, css := f.get(t, "/panel/style.css")
	values["/panel/style.css"] = snapshot{body: css, kind: "text/css; charset=utf-8"}
	return values
}

func previewNodes(t *testing.T, f *fixture) {
	t.Helper()
	for _, name := range []string{"entry-hel-01", "exit-ams-01", "relay-fra-01"} {
		token := f.enroll(t, name)
		if name == "relay-fra-01" {
			continue
		}
		desired, err := f.store.Publish(name, publication(t, name))
		require(t, err)
		require(t, f.store.UpdateStatus(name, token, wire.Status{FleetID: "test-fleet", NodeID: name, Phase: "ready", Attempt: 1,
			AcceptedRevision: desired.Revision, AppliedRevision: desired.Revision, Generation: desired.Generation, AttemptedAt: time.Now().Unix()}))
	}
}

func TestEveryPanelPageRendersWithoutPrivateCredentials(t *testing.T) {
	values := panelSnapshots(t)
	for path, value := range values {
		if strings.Contains(value.body, "<script") || strings.Contains(value.body, `"token":`) || strings.Contains(value.body, `"manifest":`) {
			t.Fatalf("page exposes executable script or private payload: %s", path)
		}
		if path != "/panel/style.css" && (!strings.Contains(value.body, "</html>") || !strings.Contains(value.body, `<html lang="ru">`)) {
			t.Fatalf("incomplete document: %s", path)
		}
	}
}
