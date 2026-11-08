package server

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"vpn-script/internal/control/fleet/registry"
)

func readJSON(w http.ResponseWriter, r *http.Request, limit int64, value any) bool {
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || kind != "application/json" {
		writeError(w, registry.ErrInvalid)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(value); err != nil {
		writeError(w, registry.ErrInvalid)
		return false
	}
	if decoder.Decode(new(any)) != io.EOF {
		writeError(w, registry.ErrInvalid)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal_error"
	switch {
	case errors.Is(err, registry.ErrUnauthorized):
		status, code = http.StatusUnauthorized, "unauthorized"
		w.Header().Set("WWW-Authenticate", `Bearer realm="nhop-fleet"`)
	case errors.Is(err, registry.ErrRevoked):
		status, code = http.StatusForbidden, "revoked"
	case errors.Is(err, registry.ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, registry.ErrConflict):
		status, code = http.StatusConflict, "conflict"
	case errors.Is(err, registry.ErrInvalid):
		status, code = http.StatusBadRequest, "invalid_request"
	case errors.Is(err, registry.ErrClosed):
		status, code = http.StatusServiceUnavailable, "unavailable"
	}
	writeJSON(w, status, map[string]string{"error": code})
}
