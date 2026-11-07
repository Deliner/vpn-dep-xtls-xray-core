package probe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"vpn-script/internal/control/probe/ipcheck"
)

func TestProbeCleanupStopsDescendantOnCancellationAndParentExit(t *testing.T) {
	for _, mode := range []string{"cancel", "parent-exit"} {
		t.Run(mode, func(t *testing.T) {
			directory := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			binary, result := probeHelper(t, "parent", directory), make(chan error, 1)
			go func() {
				local, err := localClient(ctx, binary, directory, probeRuntimeConfig("xray"))
				if local != nil {
					local.stop()
				}
				result <- err
			}()
			child := waitProbeChild(t, directory)
			if mode == "cancel" {
				cancel()
			} else if err := os.WriteFile(filepath.Join(directory, "exit"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if err == nil || errors.Is(err, context.DeadlineExceeded) || mode == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatalf("incorrect startup failure: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("client cleanup did not finish")
			}
			assertProbeClosed(t, child.Address)
		})
	}
}

func TestProbeStopIsIdempotentAndWaitsForReaping(t *testing.T) {
	directory := t.TempDir()
	local, err := localClient(context.Background(), probeHelper(t, "listener", directory), directory, probeRuntimeConfig("xray"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(local.stop)
	child := waitProbeChild(t, directory)
	var stops sync.WaitGroup
	for range 8 {
		stops.Go(local.stop)
	}
	done := make(chan struct{})
	go func() { stops.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent stop did not finish")
	}
	if local.command.ProcessState == nil {
		t.Fatal("stop returned without waiting for the process")
	}
	assertProbeClosed(t, child.Address)
}

func TestProbeRuntimeExitCancelsActiveHTTPSRead(t *testing.T) {
	directory := t.TempDir()
	local, err := localClient(context.Background(), probeHelper(t, "listener", directory), directory, probeRuntimeConfig("xray"))
	if err != nil {
		t.Fatal(err)
	}
	defer local.stop()
	started := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
	}))
	defer func() { local.stop(); server.Close() }()
	result := make(chan error, 1)
	go func() { _, err := ipcheck.Query(local.ctx, server.Client(), server.URL); result <- err }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("HTTPS read did not start")
	}
	if err := os.WriteFile(filepath.Join(directory, "exit"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		var exit *exec.ExitError
		if err == nil || !errors.As(context.Cause(local.ctx), &exit) || exit.ExitCode() != 23 {
			t.Fatalf("runtime exit did not cancel HTTP with its cause: %v / %v", err, context.Cause(local.ctx))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HTTPS survived runtime exit")
	}
}
