package panel

import (
	"net/http"

	"vpn-script/internal/control/fleet/panel/session"
	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout"
)

func New(store *registry.Store, rollouts *rollout.Manager) *Handler {
	h := &Handler{store: store, rollouts: rollouts, sessions: session.New(), mux: http.NewServeMux()}
	h.mux.HandleFunc("GET /panel/login", h.loginPage)
	h.mux.HandleFunc("POST /panel/login", h.login)
	h.mux.HandleFunc("GET /panel/style.css", stylesheet)
	h.mux.HandleFunc("GET /panel/{$}", h.authorize(h.dashboard))
	h.mux.HandleFunc("GET /panel/nodes", h.authorize(h.nodes))
	h.mux.HandleFunc("GET /panel/nodes/{node}", h.authorize(h.node))
	h.mux.HandleFunc("GET /panel/enroll", h.authorize(h.invitePage))
	h.mux.HandleFunc("POST /panel/enroll", h.authorize(h.invite))
	h.mux.HandleFunc("GET /panel/prepare", h.authorize(h.preparePage))
	h.mux.HandleFunc("POST /panel/candidates", h.authorize(h.candidate))
	h.mux.HandleFunc("POST /panel/prepare/{kind}", h.authorize(h.stageReview))
	h.mux.HandleFunc("GET /panel/review/{id}", h.authorize(h.reviewPage))
	h.mux.HandleFunc("POST /panel/review/{id}", h.authorize(h.confirm))
	h.mux.HandleFunc("GET /panel/rollouts", h.authorize(h.rolloutsPage))
	h.mux.HandleFunc("GET /panel/rollouts/{id}", h.authorize(h.rolloutPage))
	h.mux.HandleFunc("POST /panel/rollouts/{id}/cancel", h.authorize(h.cancel))
	h.mux.HandleFunc("POST /panel/nodes/{node}/revoke", h.authorize(h.revoke))
	h.mux.HandleFunc("POST /panel/logout", h.authorize(h.logout))
	return h
}
