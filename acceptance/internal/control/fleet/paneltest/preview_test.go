package paneltest

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"testing"
	"time"
)

func TestPanelPreview(t *testing.T) {
	if os.Getenv("NHOP_PANEL_PREVIEW") != "1" {
		t.Skip("opt-in, loopback-only visual preview of synthetic snapshots")
	}
	servePreview(t, panelSnapshots(t))
}

// Only synthetic rendered pages are served. There is no registry, credential
// upload, admin authority or live mutation behind this plaintext loopback server.
func servePreview(t *testing.T, pages map[string]snapshot) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:18086")
	require(t, err)
	server := &http.Server{Handler: snapshotHandler(pages), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(t.Context(), os.Interrupt)
	defer stop()
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(listener) }()
	t.Log("Read-only synthetic preview: http://127.0.0.1:18086/panel/ (expires in 10 minutes; Ctrl-C stops)")
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Minute):
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require(t, server.Shutdown(shutdown))
	if err := <-finished; err != http.ErrServerClosed {
		require(t, err)
	}
}

func snapshotHandler(pages map[string]snapshot) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Read-only fixture: no operations are performed", http.StatusMethodNotAllowed)
			return
		}
		value, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", value.kind)
		w.Header().Set("Cache-Control", "no-store")
		body := value.body
		if strings.HasPrefix(value.kind, "text/html") {
			banner := `<aside class="notice"><b>Демонстрация · Тестовые данные</b><p>Просмотр страниц. Формы не выполняют операций.</p></aside>`
			body = strings.Replace(body, "<footer", banner+"<footer", 1)
		}
		_, _ = w.Write([]byte(body))
	})
}
