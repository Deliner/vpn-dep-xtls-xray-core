package artifact

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

func downloadFixture(t *testing.T, handler http.HandlerFunc) (*http.Client, model.Asset) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)
	client := server.Client()
	client.CheckRedirect = httpsRedirect
	return client, model.Asset{URL: server.URL, Size: 8, SHA256: safeio.Hash([]byte("verified"))}
}

func TestDownloadRecoversFromTransientStatus(t *testing.T) {
	for _, status := range []int{408, 429, 500, 502, 503, 504} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			client, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) < 3 {
					w.WriteHeader(status)
					return
				}
				io.WriteString(w, "verified")
			})
			contents, err := download(context.Background(), client, asset)
			if err != nil || string(contents) != "verified" || calls.Load() != 3 {
				t.Fatalf("did not recover after two transient responses: calls=%d err=%v", calls.Load(), err)
			}
		})
	}
}

func TestDownloadStopsAtAttemptLimit(t *testing.T) {
	var calls atomic.Int32
	client, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	contents, err := download(context.Background(), client, asset)
	if err == nil || contents != nil || calls.Load() != 3 {
		t.Fatalf("unbounded or incomplete retry sequence: calls=%d err=%v", calls.Load(), err)
	}
}

func TestDownloadIntegrityFailureIsNotRetried(t *testing.T) {
	for _, body := range []string{"tampered", "short", "too-long-data"} {
		t.Run(body, func(t *testing.T) {
			var calls atomic.Int32
			client, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				io.WriteString(w, body)
			})
			contents, err := download(context.Background(), client, asset)
			if err == nil || contents != nil || calls.Load() != 1 {
				t.Fatal("integrity failure retried or returned unverified bytes")
			}
		})
	}
}

func TestDownloadRejectsHTTPBeforeSendingRequest(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		io.WriteString(w, "verified")
	}))
	defer server.Close()
	asset := model.Asset{URL: server.URL, Size: 8, SHA256: safeio.Hash([]byte("verified"))}
	if contents, err := Download(context.Background(), asset); err == nil || contents != nil || calls.Load() != 0 {
		t.Fatalf("initial HTTP request was allowed: calls=%d err=%v", calls.Load(), err)
	}
}

func TestDownloadPreservesCancellationWithoutLeakingURL(t *testing.T) {
	client, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	asset.URL += "/private?token=must-not-appear"
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := download(ctx, client, asset)
	if !errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "must-not-appear") {
		t.Fatalf("cancellation lost or private URL disclosed: %v", err)
	}
}
