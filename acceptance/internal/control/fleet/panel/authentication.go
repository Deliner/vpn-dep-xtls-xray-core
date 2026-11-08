package panel

import (
	"net/http"

	"vpn-script/internal/control/fleet/panel/session"
	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/platform/credential"
)

func (h *Handler) loginPage(w http.ResponseWriter, r *http.Request) {
	token, err := credential.NewToken()
	if err != nil {
		failure(w, access{}, err)
		return
	}
	setCookie(w, loginCookie, token, 600)
	render(w, "login", access{csrf: token}, view{Title: "Вход в Fleet"}, http.StatusOK)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	defer cleanupForm(r)
	csrf := cookieValue(r, loginCookie)
	if parseForm(w, r, 64<<10) != nil || !validCSRF(r, csrf) {
		failure(w, access{}, registry.ErrInvalid)
		return
	}
	raw, err := upload(r, "credential", 4096, false)
	var value struct {
		Token string `json:"token"`
	}
	if err != nil || decode(raw, &value) != nil || !h.store.AuthenticateAdmin(value.Token) {
		render(w, "login", access{csrf: csrf}, view{Title: "Вход в Fleet", Error: "Файл доступа не принят."}, http.StatusUnauthorized)
		return
	}
	token, _, err := h.sessions.Create()
	if err != nil {
		failure(w, access{}, err)
		return
	}
	h.sessions.Delete(cookieValue(r, sessionCookie))
	setCookie(w, loginCookie, "", -1)
	setCookie(w, sessionCookie, token, int(session.Lifetime.Seconds()))
	http.Redirect(w, r, "/panel/", http.StatusSeeOther)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request, user access) {
	h.sessions.Delete(user.token)
	setCookie(w, sessionCookie, "", -1)
	http.Redirect(w, r, "/panel/login", http.StatusSeeOther)
}
