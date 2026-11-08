package server

import (
	"net/http"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout"
)

type API struct {
	store    *registry.Store
	rollouts *rollout.Manager
	mux      *http.ServeMux
	slots    chan struct{}
}
