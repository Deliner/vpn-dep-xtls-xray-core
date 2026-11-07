package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/control/bundle"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func failingFleetRunner(t *testing.T, gates *remoteGates, installing chan struct{}) process.Runner {
	t.Helper()
	return func(ctx context.Context, timeout time.Duration, args ...string) (process.Result, error) {
		id, script := fleetNode(args), args[len(args)-1]
		if strings.Contains(script, "'--config'") {
			if id == 1 {
				return process.Result{}, fmt.Errorf("injected installer failure")
			}
			close(installing)
			<-ctx.Done()
			return process.Result{}, ctx.Err()
		}
		if strings.Contains(script, "'rm' '-rf'") && ctx.Err() != nil {
			t.Error("cleanup inherited canceled deployment context")
		}
		return gates.run(ctx, timeout, args...)
	}
}

func TestFleetFailureCancelsPeersAndRetainsLockThroughCleanup(t *testing.T) {
	directory := fleetFixture(t, 3, 2)
	gates, installing := newRemoteGates(t, 6), make(chan struct{})
	running := startFleet(t, directory, 2, gates, failingFleetRunner(t, gates, installing), io.Discard)
	fleetEvent(t, gates.started)
	fleetEvent(t, gates.started)
	gates.apply[3] <- struct{}{}
	select {
	case <-installing:
	case <-time.After(3 * time.Second):
		t.Fatal("second installer did not start")
	}
	gates.apply[1] <- struct{}{}
	if id := fleetEvent(t, gates.cleaning); id != 1 {
		t.Fatalf("expected failing node cleanup: %d", id)
	}
	gates.cleanup[1] <- struct{}{}
	if id := fleetEvent(t, gates.cleaning); id != 3 {
		t.Fatalf("canceled peer did not clean up: %d", id)
	}
	assertFleetLock(t, directory, running)
	noFleetEvent(t, gates.started)
	gates.cleanup[3] <- struct{}{}
	if err := fleetResult(t, running); err == nil {
		t.Fatal("failed deployment succeeded")
	}
	assertFailedFleetResults(t, directory)
	lock, err := bundle.Lock(directory)
	if err != nil {
		t.Fatalf("completed workers retained bundle lock: %v", err)
	}
	lock.Close()
}

func assertFleetLock(t *testing.T, directory string, running *runningFleet) {
	t.Helper()
	lock, err := bundle.Lock(directory)
	if lock != nil {
		lock.Close()
	}
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Fatalf("bundle unlocked while cleanup is active: %v", err)
	}
	select {
	case <-running.done:
		t.Fatal("deployment returned before cleanup")
	default:
	}
}

func assertFailedFleetResults(t *testing.T, directory string) {
	t.Helper()
	var results map[string]string
	if err := safeio.LoadJSON(filepath.Join(directory, "deploy-result.json"), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 6 {
		t.Fatalf("missing results: %v", results)
	}
	for id, result := range results {
		started := id == "branch00-hop01" || id == "branch01-hop01"
		if started && !strings.HasPrefix(result, "failed ") || !started && result != "not started" {
			t.Errorf("wrong result %s: %s", id, result)
		}
	}
}

func TestFleetCancelWaitsForCleanupAndReturnsCancellation(t *testing.T) {
	directory := fleetFixture(t, 1, 1)
	gates := newRemoteGates(t, 1)
	running := startFleet(t, directory, 1, gates, gates.run, io.Discard)
	fleetEvent(t, gates.started)
	gates.apply[0] <- struct{}{}
	fleetEvent(t, gates.cleaning)
	running.cancel()
	assertFleetLock(t, directory, running)
	gates.cleanup[0] <- struct{}{}
	if err := fleetResult(t, running); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was lost: %v", err)
	}
}

func TestFleetRejectsUnwritableResultBeforeSSH(t *testing.T) {
	directory := fleetFixture(t, 2, 1)
	if err := os.Mkdir(filepath.Join(directory, "deploy-result.json"), 0700); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	runner := func(context.Context, time.Duration, ...string) (process.Result, error) {
		calls.Add(1)
		return process.Result{Success: true}, nil
	}
	if err := RunConcurrent(context.Background(), directory, runner, io.Discard, 2); err == nil || calls.Load() != 0 {
		t.Fatalf("external work with unwritable journal: calls=%d err=%v", calls.Load(), err)
	}
}

func TestFleetRejectsInvalidJobsAndPriorCancellation(t *testing.T) {
	runner := func(context.Context, time.Duration, ...string) (process.Result, error) {
		t.Error("unexpected SSH call")
		return process.Result{}, nil
	}
	for _, jobs := range []int{-1, 0, 65} {
		if err := RunConcurrent(context.Background(), "nonexistent", runner, io.Discard, jobs); err == nil || !strings.Contains(err.Error(), "jobs") {
			t.Errorf("invalid jobs %d: %v", jobs, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := RunConcurrent(ctx, fleetFixture(t, 2, 1), runner, io.Discard, 2); !errors.Is(err, context.Canceled) {
		t.Fatalf("prior cancellation was lost: %v", err)
	}
}

type failedProgress struct{}

func (failedProgress) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestFleetProgressFailurePreservesAppliedResult(t *testing.T) {
	directory := fleetFixture(t, 2, 1)
	var starts atomic.Int32
	runner := func(ctx context.Context, _ time.Duration, args ...string) (process.Result, error) {
		if args[len(args)-1] == "uname -m" {
			starts.Add(1)
		}
		return successfulRemote(ctx, 0, args...)
	}
	err := RunConcurrent(context.Background(), directory, runner, failedProgress{}, 1)
	if !errors.Is(err, io.ErrClosedPipe) || starts.Load() != 1 {
		t.Fatalf("progress failure did not stop admission: starts=%d err=%v", starts.Load(), err)
	}
	var results map[string]string
	if err := safeio.LoadJSON(filepath.Join(directory, "deploy-result.json"), &results); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(results["branch00-hop00"], "applied;") || results["branch01-hop00"] != "not started" {
		t.Fatalf("completed state was lost: %v", results)
	}
}
