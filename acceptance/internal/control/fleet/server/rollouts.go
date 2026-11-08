package server

import (
	"net/http"

	"vpn-script/internal/core/fleet/wire"
)

func (api *API) rolloutRoutes() {
	api.mux.HandleFunc("POST /v1/admin/candidates/{node}", api.admin(api.candidate))
	api.mux.HandleFunc("POST /v1/admin/rollouts", api.admin(api.createRollout))
	api.mux.HandleFunc("GET /v1/admin/rollouts", api.admin(api.listRollouts))
	api.mux.HandleFunc("GET /v1/admin/rollouts/{id}", api.admin(api.getRollout))
	api.mux.HandleFunc("POST /v1/admin/rollouts/{id}/cancel", api.admin(api.cancelRollout))
}

func (api *API) candidate(w http.ResponseWriter, r *http.Request) {
	var request wire.CandidateUpload
	if !readJSON(w, r, 16<<20, &request) {
		return
	}
	value, err := api.rollouts.Stage(r.PathValue("node"), request.Manifest)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (api *API) createRollout(w http.ResponseWriter, r *http.Request) {
	var request wire.RolloutSpec
	if !readJSON(w, r, 8<<20, &request) {
		return
	}
	value, err := api.rollouts.Create(request)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (api *API) listRollouts(w http.ResponseWriter, r *http.Request) {
	values, err := api.rollouts.List()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (api *API) getRollout(w http.ResponseWriter, r *http.Request) {
	value, err := api.rollouts.Get(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (api *API) cancelRollout(w http.ResponseWriter, r *http.Request) {
	value, err := api.rollouts.Cancel(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
