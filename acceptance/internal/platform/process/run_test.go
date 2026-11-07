//go:build darwin || linux

package process_test

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func TestRunStopsBackgroundDescendants(t *testing.T) {
	for _, mode := range []string{"cancel", "timeout", "parent-exits"} {
		t.Run(mode, func(t *testing.T) { processLifecycle(t, mode) })
	}
}

func processLifecycle(t *testing.T, mode string) {
	t.Helper()
	directory := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	limit := time.Minute
	if mode == "timeout" {
		limit = 3 * time.Second
	}
	done := make(chan error, 1)
	go func() {
		_, err := process.Run(ctx, limit, os.Args[0], "nhop-process-helper", mode, directory)
		done <- err
	}()
	child := waitBackground(t, directory)
	t.Cleanup(func() { _ = syscall.Kill(child.PID, syscall.SIGKILL) })
	if mode == "cancel" {
		cancel()
	} else if mode == "parent-exits" {
		if err := os.WriteFile(filepath.Join(directory, "finish"), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-done:
		if mode != "parent-exits" && err == nil {
			t.Fatal("cancelled command reported success")
		}
	case <-time.After(6 * time.Second):
		t.Fatal("command did not finish within bound")
	}
	assertBackgroundStopped(t, child.Address)
}

func waitBackground(t *testing.T, directory string) background {
	t.Helper()
	for range 300 {
		raw, err := os.ReadFile(filepath.Join(directory, "ready.json"))
		var child background
		if err == nil && json.Unmarshal(raw, &child) == nil && child.PID > 0 {
			return child
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("background listener did not start")
	return background{}
}

func assertBackgroundStopped(t *testing.T, address string) {
	t.Helper()
	for range 100 {
		connection, err := net.DialTimeout("tcp4", address, 50*time.Millisecond)
		if err != nil {
			return
		}
		connection.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("background listener survived command completion")
}
