package panel

import (
	"net/http"

	"vpn-script/internal/control/fleet/panel/session"
	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/manifest"
)

func (h *Handler) stageReview(w http.ResponseWriter, r *http.Request, user access) {
	raw, err := upload(r, "request", 16<<20, false)
	if err != nil {
		failure(w, user, err)
		return
	}
	kind := r.PathValue("kind")
	if _, err := h.preview(kind, raw); err != nil {
		failure(w, user, err)
		return
	}
	id, err := h.sessions.Stage(user.token, kind, raw)
	if err != nil {
		failure(w, user, err)
		return
	}
	http.Redirect(w, r, "/panel/review/"+id, http.StatusSeeOther)
}

func (h *Handler) reviewPage(w http.ResponseWriter, r *http.Request, user access) {
	draft, err := h.sessions.Read(user.token, r.PathValue("id"), false)
	if err != nil {
		failure(w, user, err)
		return
	}
	if draft.Result != "" {
		http.Redirect(w, r, draft.Result, http.StatusSeeOther)
		return
	}
	value, err := h.preview(draft.Kind, draft.Body)
	if err != nil {
		failure(w, user, err)
		return
	}
	value.ID = r.PathValue("id")
	render(w, "review", user, view{Title: "Проверить и подтвердить", Section: "prepare", Review: value}, 200)
}

func (h *Handler) preview(kind string, raw []byte) (review, error) {
	switch kind {
	case "publication":
		var request wire.Publish
		if err := decode(raw, &request); err != nil {
			return review{}, err
		}
		data, err := manifest.Decode(request.Manifest)
		if err != nil {
			return review{}, registry.ErrInvalid
		}
		data, err = h.store.Preview(data.NodeID, request)
		if err != nil {
			return review{}, err
		}
		return publicationReview(data, request), nil
	case "rollout":
		var spec wire.RolloutSpec
		if h.rollouts == nil || decode(raw, &spec) != nil {
			return review{}, registry.ErrInvalid
		}
		if _, err := h.rollouts.Preview(spec); err != nil {
			return review{}, err
		}
		return review{Kind: kind, Spec: spec}, nil
	default:
		return review{}, registry.ErrInvalid
	}
}

func publicationReview(data model.NodeConfig, request wire.Publish) review {
	value := review{Kind: "publication", NodeID: data.NodeID, Generation: data.Generation, Listeners: data.Listeners,
		Expected: request.ExpectedRevision, Validity: request.ValiditySeconds, RequestID: request.RequestID, Firewall: data.Firewall}
	if data.SchemaVersion == 1 {
		value.Runtimes = []string{"xray " + data.Runtime.Version}
	}
	for _, program := range data.Programs {
		value.Runtimes = append(value.Runtimes, program.Name+" "+program.Runtime.Version)
	}
	return value
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request, user access) {
	if r.PostForm.Get("confirm") != "apply" {
		failure(w, user, registry.ErrInvalid)
		return
	}
	id := r.PathValue("id")
	draft, err := h.sessions.Read(user.token, id, true)
	if err != nil {
		failure(w, user, err)
		return
	}
	if draft.Result != "" {
		http.Redirect(w, r, draft.Result, http.StatusSeeOther)
		return
	}
	result, err := h.apply(draft)
	h.sessions.Finish(user.token, id, result)
	if err != nil {
		failure(w, user, err)
		return
	}
	http.Redirect(w, r, result, http.StatusSeeOther)
}

func (h *Handler) apply(draft session.Draft) (string, error) {
	switch draft.Kind {
	case "publication":
		var request wire.Publish
		if err := decode(draft.Body, &request); err != nil {
			return "", err
		}
		data, err := manifest.Decode(request.Manifest)
		if err != nil {
			return "", registry.ErrInvalid
		}
		if _, err := h.store.Publish(data.NodeID, request); err != nil {
			return "", err
		}
		return "/panel/nodes/" + data.NodeID, nil
	case "rollout":
		var spec wire.RolloutSpec
		if h.rollouts == nil || decode(draft.Body, &spec) != nil {
			return "", registry.ErrInvalid
		}
		value, err := h.rollouts.Create(spec)
		if err != nil {
			return "", err
		}
		return "/panel/rollouts/" + value.ID, nil
	default:
		return "", registry.ErrInvalid
	}
}
