package paneltest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vpn-script/internal/control/fleet/panel"
	"vpn-script/internal/platform/credential"
)

func TestLoginRejectsInvalidCredentialAndForeignOrigin(t *testing.T) {
	f := newFixture(t)
	_, html := f.get(t, "/panel/login")
	f.csrf = csrf(t, html)
	token, err := credential.NewToken()
	require(t, err)
	response, body := f.send(t, f.form(t, "/panel/login", nil, map[string][]byte{"credential": encoded(t, map[string]string{"token": token})}))
	if response.StatusCode != 401 || strings.Contains(body, token) || len(response.Cookies()) != 0 {
		t.Fatal("rejected administrator credential granted authority or appeared in output")
	}
	r := f.form(t, "/panel/login", nil, map[string][]byte{"credential": f.credential})
	r.Header.Set("Origin", "https://other.test")
	response, _ = f.send(t, r)
	if response.StatusCode != 400 {
		t.Fatal("foreign origin created an administrator session")
	}
	response, _ = f.get(t, "/panel/")
	if response.StatusCode != 303 || response.Header.Get("Location") != "/panel/login" {
		t.Fatal("rejected login retained browser authority")
	}
}

func TestLoginPageAllowsExternalNavigationWithoutGrantingAuthority(t *testing.T) {
	f := newFixture(t)
	r, err := http.NewRequest(http.MethodGet, f.endpoint.URL+"/panel/login", nil)
	require(t, err)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	r.Header.Set("Sec-Fetch-Mode", "navigate")
	response, html := f.send(t, r)
	if response.StatusCode != 200 || csrf(t, html) == "" {
		t.Fatal("external navigation could not reach the public login form")
	}
	response, _ = f.get(t, "/panel/")
	if response.StatusCode != 303 {
		t.Fatal("opening login granted panel authority")
	}
}

func TestBrowserAuthorityIsSeparateFromBearerAPI(t *testing.T) {
	f := newFixture(t)
	response, _ := f.get(t, "/panel/")
	if response.StatusCode != 303 || response.Header.Get("Location") != "/panel/login" {
		t.Fatal("anonymous visitor reached the panel")
	}
	response = f.login(t)
	var cookie *http.Cookie
	for _, value := range response.Cookies() {
		if value.Name == "__Host-nhop_panel" {
			cookie = value
		}
	}
	if cookie == nil || !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.MaxAge != 1800 {
		t.Fatal("browser cookie lacks authority boundaries")
	}
	response, _ = f.get(t, "/v1/admin/nodes")
	if response.StatusCode != 401 {
		t.Fatal("panel cookie granted bearer API access")
	}
	response, _ = f.send(t, f.form(t, "/panel/logout", nil, nil))
	if response.StatusCode != 303 {
		t.Fatal("logout failed")
	}
	response, _ = f.get(t, "/panel/")
	if response.StatusCode != 303 {
		t.Fatal("logout retained access")
	}
}

func TestMutationsRejectForeignOriginsAndMissingCSRF(t *testing.T) {
	f := newFixture(t)
	f.enroll(t, "node-a")
	f.login(t)
	for _, origin := range []string{"", "https://other.test", "null"} {
		r := f.form(t, "/panel/nodes/node-a/revoke", map[string]string{"confirm": "revoke"}, nil)
		r.Header.Del("Origin")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		response, _ := f.send(t, r)
		if response.StatusCode != 400 {
			t.Fatal("foreign or missing origin accepted")
		}
	}
	response, _ := f.send(t, f.form(t, "/panel/nodes/node-a/revoke", map[string]string{"csrf": "", "confirm": "revoke"}, nil))
	if response.StatusCode != 400 {
		t.Fatal("missing CSRF accepted")
	}
	node, err := f.store.Node("node-a")
	require(t, err)
	if node.Revoked {
		t.Fatal("rejected form changed the registry")
	}
}

func TestPanelEscapesSearchAndEnforcesResponsePolicy(t *testing.T) {
	f := newFixture(t)
	f.login(t)
	response, html := f.get(t, "/panel/nodes?q=%22%3E%3Cscript%3Ealert(1)%3C/script%3E")
	if response.StatusCode != 200 || strings.Contains(html, "<script>") || !strings.Contains(html, "&lt;script&gt;") {
		t.Fatal("query was not safely rendered")
	}
	if response.Header.Get("Cache-Control") != "no-store" || !strings.Contains(response.Header.Get("Content-Security-Policy"), "script-src 'none'") {
		t.Fatal("private page missing cache or script policy")
	}
	for _, path := range []string{"/panel/nodes?token=secret", "/panel/nodes?q=a&q=b", "/panel/nodes?q%7Cgroup=a", "/panel/nodes?page=-1", "/panel/prepare?q=a"} {
		response, _ := f.get(t, path)
		if response.StatusCode != 400 {
			t.Fatalf("ambiguous or unsupported query accepted: %s", path)
		}
	}
	w := httptest.NewRecorder()
	panel.New(f.store, f.rollouts).ServeHTTP(w, httptest.NewRequest("GET", "http://fleet.test/panel/login", nil))
	if w.Code != http.StatusUpgradeRequired {
		t.Fatal("panel allowed plaintext HTTP")
	}
}
