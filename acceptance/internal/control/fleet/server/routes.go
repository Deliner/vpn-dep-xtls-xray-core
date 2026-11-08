package server

import (
	"net/http"

	"vpn-script/internal/control/fleet/panel"
	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout"
)

func New(store *registry.Store, rollouts *rollout.Manager) *API {
	api := &API{store: store, rollouts: rollouts, mux: http.NewServeMux(), slots: make(chan struct{}, 8)}
	api.mux.Handle("/panel/", panel.New(store, rollouts))
	api.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/panel/", http.StatusSeeOther)
	})
	api.mux.HandleFunc("POST /v1/enroll", api.enroll)
	api.mux.HandleFunc("GET /v1/nodes/{node}/assignment", api.assignment)
	api.mux.HandleFunc("POST /v1/nodes/{node}/status", api.status)
	api.mux.HandleFunc("GET /v1/admin/nodes", api.admin(api.list))
	api.mux.HandleFunc("GET /v1/admin/nodes/{node}", api.admin(api.node))
	api.mux.HandleFunc("POST /v1/admin/invitations", api.admin(api.invite))
	api.mux.HandleFunc("POST /v1/admin/nodes/{node}/assignment", api.admin(api.publish))
	api.mux.HandleFunc("POST /v1/admin/nodes/{node}/revoke", api.admin(api.revoke))
	if rollouts != nil {
		api.rolloutRoutes()
	}
	return api
}
