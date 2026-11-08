package server

import (
	"net/http"

	"vpn-script/internal/core/fleet/wire"
)

func (api *API) enroll(w http.ResponseWriter, r *http.Request) {
	var request wire.Enrollment
	if !readJSON(w, r, 16<<10, &request) {
		return
	}
	identity, err := api.store.Enroll(request)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, identity)
}

func (api *API) assignment(w http.ResponseWriter, r *http.Request) {
	raw, err := api.store.Assignment(r.PathValue("node"), bearer(r))
	if err != nil {
		writeError(w, err)
		return
	}
	if len(raw) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
}

func (api *API) status(w http.ResponseWriter, r *http.Request) {
	var report wire.Status
	if !readJSON(w, r, 16<<10, &report) {
		return
	}
	if err := api.store.UpdateStatus(r.PathValue("node"), bearer(r), report); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
