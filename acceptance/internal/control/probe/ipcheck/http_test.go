package ipcheck

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIPQueryIgnoresShellProxyAndCurlSettings(t *testing.T) {
	var unrelated atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		unrelated.Add(1)
		fmt.Fprint(w, "198.51.100.99")
	}))
	defer proxy.Close()
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"} {
		t.Setenv(name, proxy.URL)
	}
	t.Setenv("NO_PROXY", "*")
	t.Setenv("no_proxy", "*")
	fixture := probeHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "198.51.100.42\n")
	}))
	hostileCurlConfig(t, fixture)
	actual, err := Query(context.Background(), trustedIPClient(t, fixture), fixture.endpoint)
	if err != nil || actual != "198.51.100.42" || unrelated.Load() != 0 {
		t.Fatalf("probe inherited shell networking/output policy: ip=%q unrelated=%d err=%v", actual, unrelated.Load(), err)
	}
	select {
	case destination := <-fixture.observed:
		if destination != strings.TrimPrefix(fixture.endpoint, "https://") {
			t.Fatalf("hostname was not sent to SOCKS: %q", destination)
		}
	case <-time.After(time.Second):
		t.Fatal("SOCKS proxy did not receive the request")
	}
}

func TestIPQueryRequiresTrustedAuthorityAndHostname(t *testing.T) {
	for _, fault := range []string{"authority", "hostname"} {
		t.Run(fault, func(t *testing.T) {
			var hits atomic.Int32
			fixture := probeHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits.Add(1)
				fmt.Fprint(w, "198.51.100.42")
			}))
			client, endpoint := trustedIPClient(t, fixture), fixture.endpoint
			if fault == "authority" {
				client.Transport.(*http.Transport).TLSClientConfig.RootCAs = nil
			} else {
				endpoint = strings.Replace(endpoint, "ip-probe.invalid", "wrong-name.invalid", 1)
			}
			actual, err := Query(context.Background(), client, endpoint)
			if err == nil || actual != "" || hits.Load() != 0 {
				t.Fatalf("TLS refusal failed: ip=%q hits=%d err=%v", actual, hits.Load(), err)
			}
		})
	}
}

func TestIPQueryDoesNotFollowRedirect(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected.Add(1)
		fmt.Fprint(w, "198.51.100.99")
	}))
	defer target.Close()
	fixture := probeHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(http.StatusFound)
		fmt.Fprint(w, "198.51.100.42")
	}))
	actual, err := Query(context.Background(), trustedIPClient(t, fixture), fixture.endpoint)
	if err == nil || actual != "" || redirected.Load() != 0 {
		t.Fatalf("redirect accepted: ip=%q followed=%d err=%v", actual, redirected.Load(), err)
	}
}

func TestIPQueryResponseBoundAndLiteralParsing(t *testing.T) {
	cases := []struct{ name, body, expected string }{
		{"ipv4", "198.51.100.42\n", "198.51.100.42"},
		{"ipv6", "2001:0db8:0000::42\n", "2001:db8::42"},
		{"mapped", "::ffff:198.51.100.42", "198.51.100.42"},
		{"exact-limit", "198.51.100.42" + strings.Repeat(" ", ipResponseLimit-13), "198.51.100.42"},
		{"over-limit", "198.51.100.42" + strings.Repeat(" ", ipResponseLimit-12), ""},
		{"zone", "fe80::1%eth0", ""},
		{"multiple", "198.51.100.42\n198.51.100.43", ""},
		{"json", "{\"ip\":\"198.51.100.42\"}", ""},
		{"empty", "", ""},
		{"truncated-prefix", "198.51.100.42" + strings.Repeat(" ", 65536) + "trailing content", ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := probeHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprint(w, test.body)
			}))
			actual, err := Query(context.Background(), trustedIPClient(t, fixture), fixture.endpoint)
			if (err == nil) != (test.expected != "") || actual != test.expected {
				t.Fatalf("body contract: ip=%q expected=%q err=%v", actual, test.expected, err)
			}
		})
	}
}

func TestIPQueryRejectsNonOKStatusEvenWithValidIP(t *testing.T) {
	for _, status := range []int{201, 204, 206, 301, 403, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			fixture := probeHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, "198.51.100.42")
			}))
			actual, err := Query(context.Background(), trustedIPClient(t, fixture), fixture.endpoint)
			if err == nil || actual != "" {
				t.Fatalf("HTTP failure accepted as exit IP: %q %v", actual, err)
			}
		})
	}
}

func TestIPQueryPreservesExplicitEndpointAuthentication(t *testing.T) {
	fixture := probeHTTPFixture(t, http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		user, password, ok := request.BasicAuth()
		if !ok || user != "probe-user" || password != "test:credential" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, "198.51.100.42")
	}))
	endpoint := strings.Replace(fixture.endpoint, "https://", "https://probe-user:test%3Acredential@", 1)
	actual, err := Query(context.Background(), trustedIPClient(t, fixture), endpoint)
	if err != nil || actual != "198.51.100.42" {
		t.Fatalf("explicit endpoint authentication changed: %q %v", actual, err)
	}
}
