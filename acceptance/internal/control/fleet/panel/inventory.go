package panel

import (
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/core/fleet/wire"
)

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request, user access) {
	nodes, err := h.store.List()
	if err != nil {
		failure(w, user, err)
		return
	}
	rollouts, err := h.rolloutList()
	if err != nil {
		failure(w, user, err)
		return
	}
	data := view{Title: "Обзор флота", Section: "overview", Total: len(nodes)}
	for _, node := range nodes {
		row := describeNode(node)
		if row.Tone == "good" {
			data.Recent++
		} else if node.Enrolled && !node.Revoked {
			data.Attention++
		}
		if len(data.Nodes) < 6 {
			data.Nodes = append(data.Nodes, row)
		}
	}
	for _, value := range rollouts {
		if value.Phase == "running" {
			data.Active++
		}
	}
	data.Rollouts = rollouts[:min(len(rollouts), 4)]
	render(w, "overview", user, data, 200)
}

func (h *Handler) nodes(w http.ResponseWriter, r *http.Request, user access) {
	values, err := h.store.List()
	if err != nil {
		failure(w, user, err)
		return
	}
	data := view{Title: "Узлы", Section: "nodes", Query: r.URL.Query().Get("q"), Group: r.URL.Query().Get("group")}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value.NodeID), strings.ToLower(data.Query)) && (data.Group == "" || value.Group == data.Group) {
			data.Nodes = append(data.Nodes, describeNode(value))
		}
	}
	start, end, err := paginate(r, len(data.Nodes), &data)
	if err != nil {
		failure(w, user, err)
		return
	}
	data.Nodes = data.Nodes[start:end]
	render(w, "nodes", user, data, 200)
}

func (h *Handler) node(w http.ResponseWriter, r *http.Request, user access) {
	node, err := h.store.Node(r.PathValue("node"))
	if err != nil {
		failure(w, user, err)
		return
	}
	render(w, "node", user, view{Title: node.NodeID, Section: "nodes", Node: describeNode(node)}, 200)
}

func describeNode(node wire.Node) nodeRow {
	row := nodeRow{Node: node, Label: "Ожидает отчёта", Tone: "muted"}
	switch {
	case node.Revoked:
		row.Label = "Доступ отозван"
	case !node.Enrolled:
		row.Label = "Приглашён"
	case node.LastSeen == 0 || time.Now().Unix()-node.LastSeen > 90:
		row.Label, row.Tone = "Нет свежей связи", "warn"
	case node.Status != nil && node.Status.Phase == "failed":
		row.Label, row.Tone = "Ошибка применения", "bad"
	case node.Status != nil && node.Status.Phase == "ready" && node.Status.Generation == node.Desired.Generation && node.Desired.Revision > 0:
		row.Label, row.Tone = "Применён", "good"
	default:
		row.Label, row.Tone = "Ожидает применения", "warn"
	}
	return row
}

func (h *Handler) rolloutList() ([]wire.Rollout, error) {
	if h.rollouts == nil {
		return nil, nil
	}
	values, err := h.rollouts.List()
	sort.Slice(values, func(i, j int) bool { return values[i].CreatedAt > values[j].CreatedAt })
	return values, err
}

func (h *Handler) rolloutsPage(w http.ResponseWriter, r *http.Request, user access) {
	values, err := h.rolloutList()
	if err != nil {
		failure(w, user, err)
		return
	}
	data := view{Title: "Обновления", Section: "rollouts", Query: r.URL.Query().Get("q")}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value.ID), strings.ToLower(data.Query)) {
			data.Rollouts = append(data.Rollouts, value)
		}
	}
	start, end, err := paginate(r, len(data.Rollouts), &data)
	if err != nil {
		failure(w, user, err)
		return
	}
	data.Rollouts = data.Rollouts[start:end]
	render(w, "rollouts", user, data, 200)
}

func (h *Handler) rolloutPage(w http.ResponseWriter, r *http.Request, user access) {
	if h.rollouts == nil {
		failure(w, user, registry.ErrNotFound)
		return
	}
	value, err := h.rollouts.Get(r.PathValue("id"))
	if err != nil {
		failure(w, user, err)
		return
	}
	render(w, "rollout", user, view{Title: value.ID, Section: "rollouts", Rollout: value}, 200)
}

func paginate(r *http.Request, count int, data *view) (int, int, error) {
	page := 1
	if raw := r.URL.Query().Get("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 200000 {
			return 0, 0, registry.ErrInvalid
		}
		page = value
	}
	data.Total, data.Pages = count, max(1, (count+49)/50)
	data.Page = min(page, data.Pages)
	if data.Page > 1 {
		data.Previous = pageLink(r, data.Page-1)
	}
	if data.Page < data.Pages {
		data.Next = pageLink(r, data.Page+1)
	}
	start := (data.Page - 1) * 50
	return start, min(start+50, count), nil
}

func pageLink(r *http.Request, page int) string {
	values := url.Values{"page": {strconv.Itoa(page)}, "q": {r.URL.Query().Get("q")}, "group": {r.URL.Query().Get("group")}}
	return r.URL.Path + "?" + values.Encode()
}
