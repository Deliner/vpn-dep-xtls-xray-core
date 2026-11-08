// Package panel serves the Fleet browser interface. Registry and rollout keep
// authority over durable changes; browser sessions only hold review requests.
package panel

import (
	"net/http"

	"vpn-script/internal/control/fleet/panel/session"
	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/fleet/rollout"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
)

type Handler struct {
	store    *registry.Store
	rollouts *rollout.Manager
	sessions *session.Store
	mux      *http.ServeMux
}

type access struct {
	token string
	csrf  string
}

type view struct {
	Title, Section, CSRF, Error, Message string
	Authenticated                        bool
	Nodes                                []nodeRow
	Node                                 nodeRow
	Rollouts                             []wire.Rollout
	Rollout                              wire.Rollout
	Candidate                            wire.Candidate
	Review                               review
	Total, Recent, Attention, Active     int
	Query, Group, Previous, Next         string
	Page, Pages                          int
}

type nodeRow struct {
	wire.Node
	Label, Tone string
}

type review struct {
	ID, Kind, NodeID, Generation, RequestID string
	Firewall                                string
	Runtimes                                []string
	Expected                                uint64
	Validity                                int64
	Listeners                               []model.Listener
	Spec                                    wire.RolloutSpec
}
