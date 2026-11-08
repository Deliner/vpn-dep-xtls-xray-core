package server_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpn-script/internal/control/fleet/server"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/credential"
)

func TestHTTPSSeparatesNodeAndAdministratorAuthority(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	invite, token := f.enroll(t, "node-a")
	_, other := f.enroll(t, "node-b")
	_, err := f.client.List(ctx, token)
	requireHTTP(t, err, http.StatusUnauthorized)
	_, err = f.client.Assignment(ctx, "node-a", other)
	requireHTTP(t, err, http.StatusUnauthorized)
	_, err = f.client.Assignment(ctx, "node-a", f.admin)
	requireHTTP(t, err, http.StatusUnauthorized)
	request := wire.Enrollment{NodeID: "node-a", Token: invite.Token, Credential: token}
	_, err = f.client.Enroll(ctx, request)
	require(t, err)
	request.Credential, err = credential.NewToken()
	require(t, err)
	_, err = f.client.Enroll(ctx, request)
	requireHTTP(t, err, http.StatusUnauthorized)
	requireHTTP(t, f.client.Revoke(ctx, "node-b", token), http.StatusUnauthorized)
}

func TestRolloutRoutesRequireAdminAndKeepCancellationIdempotent(t *testing.T) {
	f := newFixture(t)
	spec, tokens := f.rolloutPlan(t)
	_, err := f.client.StageCandidate(t.Context(), "node-a", tokens["node-a"], publication(t, "node-a").Manifest)
	requireHTTP(t, err, http.StatusUnauthorized)
	_, err = f.client.CreateRollout(t.Context(), tokens["node-a"], spec)
	requireHTTP(t, err, http.StatusUnauthorized)
	_, err = f.client.CreateRollout(t.Context(), f.admin, spec)
	require(t, err)
	_, err = f.client.Rollout(t.Context(), spec.ID, tokens["node-a"])
	requireHTTP(t, err, http.StatusUnauthorized)
	_, err = f.client.CancelRollout(t.Context(), spec.ID, tokens["node-a"])
	requireHTTP(t, err, http.StatusUnauthorized)
	values, err := f.client.Rollouts(t.Context(), f.admin)
	require(t, err)
	if len(values) != 1 || len(values[0].Nodes) != 0 || values[0].TotalNodes != 2 {
		t.Fatal("rollout list is not a bounded summary")
	}
	for range 2 {
		value, err := f.client.CancelRollout(t.Context(), spec.ID, f.admin)
		require(t, err)
		if value.Phase != "canceled" || value.PublishedNodes != 0 {
			t.Fatal("idempotent cancellation changed node assignments")
		}
	}
}

func TestAPIRejectsPlainHTTPAndForeignOrigins(t *testing.T) {
	f := newFixture(t)
	api := server.New(f.store, nil)
	for _, address := range []string{"http://fleet.test/v1/admin/nodes", "https://fleet.test/v1/admin/nodes?token=ignored"} {
		r := httptest.NewRequest(http.MethodGet, address, nil)
		r.Header.Set("Authorization", "Bearer "+f.admin)
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		if w.Code < 400 {
			t.Fatal("HTTP or query-bearing request accepted")
		}
	}
	r := httptest.NewRequest(http.MethodGet, "https://fleet.test/v1/admin/nodes", nil)
	r.Header.Set("Authorization", "Bearer "+f.admin)
	r.Header.Set("Origin", "https://other.test")
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("foreign-origin request accepted or caching allowed")
	}
}

func TestAPIRejectsAmbiguousAuthorizationAndMalformedBodies(t *testing.T) {
	f := newFixture(t)
	api := server.New(f.store, nil)
	r := httptest.NewRequest(http.MethodGet, "https://fleet.test/v1/admin/nodes", nil)
	r.Header.Add("Authorization", "Bearer "+f.admin)
	r.Header.Add("Authorization", "Bearer "+f.admin)
	w := httptest.NewRecorder()
	api.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatal("multiple authorization headers accepted")
	}
	for _, body := range []string{`{} {}`, `{"unexpected":true}`, strings.Repeat(" ", 17<<10) + `{}`} {
		r := httptest.NewRequest(http.MethodPost, "https://fleet.test/v1/admin/invitations", bytes.NewBufferString(body))
		r.Header.Set("Authorization", "Bearer "+f.admin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		api.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest || strings.Contains(w.Body.String(), f.admin) {
			t.Fatal("invalid request accepted or credential leaked")
		}
	}
}
