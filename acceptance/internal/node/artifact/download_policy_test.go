package artifact

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"vpn-script/internal/core/model"
)

func TestDownloadRetriesTruncatedTransfer(t *testing.T) {
	var calls atomic.Int32
	client, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "8")
		if calls.Add(1) == 1 {
			io.WriteString(w, "ver")
			return
		}
		io.WriteString(w, "verified")
	})
	contents, err := download(context.Background(), client, asset)
	if err != nil || string(contents) != "verified" || calls.Load() != 2 {
		t.Fatalf("incomplete transfer was not retried safely: %v, calls=%d", err, calls.Load())
	}
}

func TestDownloadDoesNotRetryPermanentHTTPStatus(t *testing.T) {
	for _, status := range []int{401, 403, 404, 501, 505} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			client, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
			})
			if contents, err := download(context.Background(), client, asset); err == nil || contents != nil || calls.Load() != 1 {
				t.Fatalf("permanent failure retried: %v, calls=%d", err, calls.Load())
			}
		})
	}
}

type attemptCounter struct {
	base  http.RoundTripper
	calls atomic.Int32
}

func (counter *attemptCounter) RoundTrip(request *http.Request) (*http.Response, error) {
	counter.calls.Add(1)
	return counter.base.RoundTrip(request)
}

func TestDownloadDoesNotRetryCertificateFailure(t *testing.T) {
	_, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) { t.Error("untrusted TLS was accepted") })
	client := downloadClient()
	t.Cleanup(client.Transport.(*http.Transport).CloseIdleConnections)
	counter := &attemptCounter{base: client.Transport}
	client.Transport = counter
	_, err := download(context.Background(), client, asset)
	var certificate x509.UnknownAuthorityError
	if !errors.As(err, &certificate) || counter.calls.Load() != 1 {
		t.Fatalf("TLS trust failure retried or its cause lost: %v, calls=%d", err, counter.calls.Load())
	}
}

func TestDownloadRejectsDowngradeWithoutContactingTarget(t *testing.T) {
	var targetCalls, sourceCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls.Add(1) }))
	defer target.Close()
	client, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		sourceCalls.Add(1)
		http.Redirect(w, r, target.URL, http.StatusFound)
	})
	if contents, err := download(context.Background(), client, asset); err == nil || contents != nil {
		t.Fatal("HTTPS downgrade accepted")
	}
	if sourceCalls.Load() != 1 || targetCalls.Load() != 0 {
		t.Fatal("unsafe redirect was followed or retried")
	}
}

func TestDownloadRejectsInvalidMetadataBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	client, original := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) })
	for _, change := range []func(*model.Asset){
		func(a *model.Asset) { a.Size = 0 }, func(a *model.Asset) { a.Size = -1 },
		func(a *model.Asset) { a.Size = math.MaxInt64 }, func(a *model.Asset) { a.Size = 150_000_001 },
		func(a *model.Asset) { a.SHA256 = "invalid" }, func(a *model.Asset) { a.URL += "#fragment" },
		func(a *model.Asset) { a.URL = "https://user:password@example.invalid/asset" },
	} {
		asset := original
		change(&asset)
		if contents, err := download(context.Background(), client, asset); err == nil || contents != nil {
			t.Fatal("invalid asset metadata accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid metadata caused network activity")
	}
}

func TestDownloadRespectsRetryAfter(t *testing.T) {
	var calls atomic.Int32
	var first atomic.Pointer[time.Time]
	client, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			started := time.Now()
			first.Store(&started)
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if time.Since(*first.Load()) < time.Second {
			t.Error("retried before the server's deadline")
		}
		io.WriteString(w, "verified")
	})
	contents, err := download(context.Background(), client, asset)
	if err != nil || string(contents) != "verified" || calls.Load() != 2 {
		t.Fatalf("Retry-After did not recover: %v", err)
	}
}

func TestDownloadBudgetInterruptsRetryAfter(t *testing.T) {
	var calls atomic.Int32
	client, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "9223372036854775807")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := download(ctx, client, asset)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 3*time.Second || calls.Load() != 1 {
		t.Fatalf("retry exceeded the caller budget: %v, calls=%d", err, calls.Load())
	}
}

func TestDownloadBudgetInterruptsRedirectWait(t *testing.T) {
	var calls atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer target.Close()
	client, asset := downloadFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", time.Now().Add(time.Hour).UTC().Format(http.TimeFormat))
		http.Redirect(w, r, target.URL, http.StatusFound)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := download(ctx, client, asset)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 3*time.Second || calls.Load() != 0 {
		t.Fatalf("redirect ignored Retry-After or cancellation: %v, calls=%d", err, calls.Load())
	}
}
