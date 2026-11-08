package server

import (
	"net/http"
	"strings"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/platform/credential"
)

func (api *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if r.TLS == nil {
		writeJSON(w, http.StatusUpgradeRequired, map[string]string{"error": "tls_required"})
		return
	}
	if r.URL.RawQuery != "" && !strings.HasPrefix(r.URL.Path, "/panel/") || r.Header.Get("Origin") != "" && !strings.EqualFold(r.Header.Get("Origin"), "https://"+r.Host) {
		writeError(w, registry.ErrInvalid)
		return
	}
	select {
	case api.slots <- struct{}{}:
		defer func() { <-api.slots }()
		api.mux.ServeHTTP(w, r)
	default:
		w.Header().Set("Retry-After", "2")
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "busy"})
	}
}

func (api *API) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !api.store.AuthenticateAdmin(bearer(r)) {
			writeError(w, registry.ErrUnauthorized)
			return
		}
		next(w, r)
	}
}

func bearer(r *http.Request) string {
	values := r.Header.Values("Authorization")
	if len(values) != 1 || len(values[0]) < 8 || !strings.EqualFold(values[0][:7], "Bearer ") {
		return ""
	}
	token := values[0][7:]
	if !credential.ValidToken(token) {
		return ""
	}
	return token
}
