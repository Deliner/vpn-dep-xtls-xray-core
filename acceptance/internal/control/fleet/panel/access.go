package panel

import (
	"net/http"
	"net/url"

	"vpn-script/internal/control/fleet/registry"
)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'none'; style-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	if r.TLS == nil {
		http.Error(w, "Требуется HTTPS", http.StatusUpgradeRequired)
		return
	}
	if !validQuery(r) || !validOrigin(r) {
		failure(w, access{}, registry.ErrInvalid)
		return
	}
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) authorize(next func(http.ResponseWriter, *http.Request, access)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := cookieValue(r, sessionCookie)
		identity, ok := h.sessions.Lookup(token)
		if !ok {
			http.Redirect(w, r, "/panel/login", http.StatusSeeOther)
			return
		}
		user := access{token: token, csrf: identity.CSRF}
		if r.Method == http.MethodPost {
			defer cleanupForm(r)
			if parseForm(w, r, 17<<20) != nil || !validCSRF(r, user.csrf) {
				failure(w, user, registry.ErrInvalid)
				return
			}
		}
		next(w, r, user)
	}
}

func validOrigin(r *http.Request) bool {
	origins := r.Header.Values("Origin")
	if len(origins) > 1 || len(origins) == 1 && origins[0] != "https://"+r.Host {
		return false
	}
	if r.Method != http.MethodPost {
		return true
	}
	if len(origins) != 1 {
		return false
	}
	site := r.Header.Get("Sec-Fetch-Site")
	return site == "" || site == "same-origin" || site == "none"
}

func validQuery(r *http.Request) bool {
	if len(r.URL.RawQuery) > 1024 {
		return false
	}
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return false
	}
	if len(values) == 0 {
		return true
	}
	if r.Method != http.MethodGet || r.URL.Path != "/panel/nodes" && r.URL.Path != "/panel/rollouts" {
		return false
	}
	for name, items := range values {
		if len(items) != 1 || name != "q" && name != "group" && name != "page" {
			return false
		}
	}
	return true
}
