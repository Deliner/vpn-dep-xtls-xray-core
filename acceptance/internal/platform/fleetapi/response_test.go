package fleetapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
)

func TestRejectsOversizedMalformedAndNonJSONResponses(t *testing.T) {
	cases := []struct{ kind, body string }{
		{"application/json", strings.Repeat(" ", 17<<10) + `{}`},
		{"application/json", `{"fleet_id":"a","node_id":"b"} {}`},
		{"application/json", `{"fleet_id":"a","node_id":"b","unknown":true}`},
		{"text/html", `{"fleet_id":"a","node_id":"b"}`},
	}
	for _, test := range cases {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", test.kind)
			w.Write([]byte(test.body))
		}))
		client := trustedClient(t, server)
		_, err := client.Enroll(context.Background(), wire.Enrollment{})
		client.Close()
		server.Close()
		if err == nil {
			t.Fatal("invalid response accepted")
		}
	}
}

func TestCancellationInterruptsWaitingRequest(t *testing.T) {
	entered := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	}))
	defer server.Close()
	client := trustedClient(t, server)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := client.Assignment(ctx, "node-a", "")
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not reach the server")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled request succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not release the request")
	}
}

func TestServerErrorsDoNotExposeResponseBody(t *testing.T) {
	const secret = "private-diagnostic-or-credential"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, secret, http.StatusInternalServerError)
	}))
	defer server.Close()
	client := trustedClient(t, server)
	_, err := client.Assignment(context.Background(), "node-a", "")
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe server error: %v", err)
	}
}
