package ipcheck

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIPQueryCancellationClosesResponseBody(t *testing.T) {
	started := make(chan struct{})
	fixture := probeHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, result := trustedIPClient(t, fixture), make(chan error, 1)
	go func() {
		_, err := Query(ctx, client, fixture.endpoint)
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("origin did not begin response")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("response body lost cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled response remained blocked")
	}
}

func TestIPQueryHasNoDirectFallbackWhenSOCKSStalls(t *testing.T) {
	var direct atomic.Int32
	fixture := probeHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		direct.Add(1)
		fmt.Fprint(w, "198.51.100.42")
	}))
	stalled, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer stalled.Close()
	client := NewClient(stalled.Addr().(*net.TCPAddr).Port)
	client.Transport.(*http.Transport).TLSClientConfig = trustedIPClient(t, fixture).Transport.(*http.Transport).TLSClientConfig
	client.Transport.(*http.Transport).TLSClientConfig.ServerName = "ip-probe.invalid"
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	endpoint := strings.Replace(fixture.endpoint, "ip-probe.invalid", "127.0.0.1", 1)
	actual, err := Query(ctx, client, endpoint)
	if !errors.Is(err, context.DeadlineExceeded) || actual != "" || direct.Load() != 0 {
		t.Fatalf("SOCKS failure fell back to direct: ip=%q hits=%d err=%v", actual, direct.Load(), err)
	}
}

func TestIPEndpointRejectsUnsafeOrAmbiguousURLs(t *testing.T) {
	for _, endpoint := range []string{"", "/relative", "http://example.invalid", "file:///tmp/ip",
		"https:opaque", "https://", "https://example.invalid/#fragment",
		"https://example.invalid:0", "https://example.invalid:65536", "https://example.invalid:invalid"} {
		if _, err := Endpoint(endpoint); err == nil {
			t.Errorf("invalid endpoint accepted: %q", endpoint)
		}
	}
	for _, endpoint := range []string{"https://example.invalid", "https://user:password@example.invalid", "https://[2001:db8::1]:8443/ip?format=text"} {
		if _, err := Endpoint(endpoint); err != nil {
			t.Errorf("valid endpoint rejected: %q: %v", endpoint, err)
		}
	}
}

func TestIPQueryRejectsOversizedHeadersAndIncompleteBody(t *testing.T) {
	for _, fault := range []string{"headers", "truncated-body"} {
		t.Run(fault, func(t *testing.T) {
			fixture := probeHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if fault == "headers" {
					w.Header().Set("X-Oversized", strings.Repeat("x", 17*1024))
				} else {
					w.Header().Set("Content-Length", "100")
				}
				fmt.Fprint(w, "198.51.100.42")
			}))
			actual, err := Query(context.Background(), trustedIPClient(t, fixture), fixture.endpoint)
			if err == nil || actual != "" {
				t.Fatalf("malformed response confirmed an IP: %q %v", actual, err)
			}
		})
	}
}
