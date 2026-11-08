package server

import (
	"net/http"

	"vpn-script/internal/core/fleet/wire"
)

func (api *API) list(w http.ResponseWriter, r *http.Request) {
	value, err := api.store.List()
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (api *API) node(w http.ResponseWriter, r *http.Request) {
	value, err := api.store.Node(r.PathValue("node"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (api *API) invite(w http.ResponseWriter, r *http.Request) {
	var request wire.Invite
	if !readJSON(w, r, 16<<10, &request) {
		return
	}
	value, err := api.store.Invite(request)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}

func (api *API) publish(w http.ResponseWriter, r *http.Request) {
	var request wire.Publish
	if !readJSON(w, r, 16<<20, &request) {
		return
	}
	value, err := api.store.Publish(r.PathValue("node"), request)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (api *API) revoke(w http.ResponseWriter, r *http.Request) {
	if err := api.store.Revoke(r.PathValue("node")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
