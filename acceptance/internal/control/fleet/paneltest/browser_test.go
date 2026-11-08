package paneltest

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"testing"
)

func (f *fixture) get(t *testing.T, path string) (*http.Response, string) {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, f.endpoint.URL+path, nil)
	require(t, err)
	return f.send(t, r)
}

func (f *fixture) send(t *testing.T, r *http.Request) (*http.Response, string) {
	t.Helper()
	response, err := f.client.Do(r)
	require(t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 20<<20))
	require(t, err)
	return response, string(raw)
}

func (f *fixture) form(t *testing.T, path string, fields map[string]string, files map[string][]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if _, exists := fields["csrf"]; !exists && f.csrf != "" {
		require(t, writer.WriteField("csrf", f.csrf))
	}
	for name, value := range fields {
		require(t, writer.WriteField(name, value))
	}
	for name, value := range files {
		part, err := writer.CreateFormFile(name, name+".json")
		require(t, err)
		_, err = part.Write(value)
		require(t, err)
	}
	require(t, writer.Close())
	r, err := http.NewRequest(http.MethodPost, f.endpoint.URL+path, &body)
	require(t, err)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Origin", f.endpoint.URL)
	return r
}

func csrf(t *testing.T, html string) string {
	t.Helper()
	match := regexp.MustCompile(`name="csrf" value="([A-Za-z0-9_-]{43})"`).FindStringSubmatch(html)
	if len(match) != 2 {
		t.Fatal("page has no CSRF token")
	}
	return match[1]
}

func (f *fixture) login(t *testing.T) *http.Response {
	t.Helper()
	_, html := f.get(t, "/panel/login")
	f.csrf = csrf(t, html)
	response, _ := f.send(t, f.form(t, "/panel/login", nil, map[string][]byte{"credential": f.credential}))
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("login status %d", response.StatusCode)
	}
	_, html = f.get(t, "/panel/")
	f.csrf = csrf(t, html)
	return response
}
