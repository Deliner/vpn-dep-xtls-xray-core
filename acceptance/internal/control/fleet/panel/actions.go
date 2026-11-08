package panel

import (
	"encoding/json"
	"net/http"
	"strconv"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/fleet/pull"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/fleetapi"
)

func (h *Handler) invitePage(w http.ResponseWriter, r *http.Request, user access) {
	render(w, "enroll", user, view{Title: "Добавить узел", Section: "nodes"}, 200)
}

func (h *Handler) preparePage(w http.ResponseWriter, r *http.Request, user access) {
	render(w, "prepare", user, view{Title: "Подготовить обновление", Section: "prepare"}, 200)
}

func (h *Handler) invite(w http.ResponseWriter, r *http.Request, user access) {
	validity, err := strconv.ParseInt(r.PostForm.Get("validity"), 10, 64)
	if err != nil {
		failure(w, user, registry.ErrInvalid)
		return
	}
	ca, err := upload(r, "ca", 1<<20, true)
	if err != nil {
		failure(w, user, err)
		return
	}
	endpoint := "https://" + r.Host
	client, err := fleetapi.New(endpoint, ca)
	if err != nil {
		failure(w, user, registry.ErrInvalid)
		return
	}
	client.Close()
	value, err := h.store.Invite(wire.Invite{NodeID: r.PostForm.Get("node"), Group: r.PostForm.Get("group"), ValiditySeconds: validity})
	if err != nil {
		failure(w, user, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="bootstrap-`+value.NodeID+`.json"`)
	_ = json.NewEncoder(w).Encode(pull.Bootstrap{Version: 1, Endpoint: endpoint, CAPEM: ca, Invitation: value})
}

func (h *Handler) candidate(w http.ResponseWriter, r *http.Request, user access) {
	if h.rollouts == nil {
		failure(w, user, registry.ErrNotFound)
		return
	}
	raw, err := upload(r, "manifest", model.MaxNodeConfigSize, false)
	if err != nil {
		failure(w, user, err)
		return
	}
	data, err := manifest.Decode(raw)
	if err != nil {
		failure(w, user, registry.ErrInvalid)
		return
	}
	value, err := h.rollouts.Stage(data.NodeID, raw)
	if err != nil {
		failure(w, user, err)
		return
	}
	render(w, "candidate", user, view{Title: "Кандидат сохранён", Section: "prepare", Candidate: value}, 200)
}

func (h *Handler) revoke(w http.ResponseWriter, r *http.Request, user access) {
	if r.PostForm.Get("confirm") != "revoke" {
		failure(w, user, registry.ErrInvalid)
		return
	}
	if err := h.store.Revoke(r.PathValue("node")); err != nil {
		failure(w, user, err)
		return
	}
	http.Redirect(w, r, "/panel/nodes/"+r.PathValue("node"), http.StatusSeeOther)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request, user access) {
	if h.rollouts == nil || r.PostForm.Get("confirm") != "cancel" {
		failure(w, user, registry.ErrInvalid)
		return
	}
	value, err := h.rollouts.Cancel(r.PathValue("id"))
	if err != nil {
		failure(w, user, err)
		return
	}
	http.Redirect(w, r, "/panel/rollouts/"+value.ID, http.StatusSeeOther)
}
