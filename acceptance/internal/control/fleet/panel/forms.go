package panel

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"time"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/platform/credential"
)

const sessionCookie = "__Host-nhop_panel"
const loginCookie = "__Host-nhop_login"

func parseForm(w http.ResponseWriter, r *http.Request, limit int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	kind, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return registry.ErrInvalid
	}
	switch kind {
	case "multipart/form-data":
		err = r.ParseMultipartForm(limit)
	case "application/x-www-form-urlencoded":
		err = r.ParseForm()
	default:
		return registry.ErrInvalid
	}
	if err != nil {
		return registry.ErrInvalid
	}
	for _, values := range r.PostForm {
		if len(values) != 1 || len(values[0]) > 4096 {
			return registry.ErrInvalid
		}
	}
	return nil
}

func cleanupForm(r *http.Request) {
	if r.MultipartForm != nil {
		_ = r.MultipartForm.RemoveAll()
	}
}

func upload(r *http.Request, name string, limit int64, optional bool) ([]byte, error) {
	if r.MultipartForm == nil || len(r.MultipartForm.File[name]) == 0 {
		if optional {
			return nil, nil
		}
		return nil, registry.ErrInvalid
	}
	files := r.MultipartForm.File[name]
	if len(files) != 1 || files[0].Size > limit {
		return nil, registry.ErrInvalid
	}
	file, err := files[0].Open()
	if err != nil {
		return nil, registry.ErrInvalid
	}
	defer file.Close()
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(raw)) > limit {
		return nil, registry.ErrInvalid
	}
	return raw, nil
}

func decode(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil || d.Decode(new(any)) != io.EOF {
		return registry.ErrInvalid
	}
	return nil
}

func cookieValue(r *http.Request, name string) string {
	values := r.CookiesNamed(name)
	if len(values) != 1 || !credential.ValidToken(values[0].Value) {
		return ""
	}
	return values[0].Value
}

func setCookie(w http.ResponseWriter, name, value string, seconds int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", Secure: true,
		HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: seconds,
		Expires: time.Now().Add(time.Duration(seconds) * time.Second)})
}

func validCSRF(r *http.Request, expected string) bool {
	provided := r.PostForm.Get("csrf")
	return credential.ValidToken(expected) && credential.ValidToken(provided) && subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}
