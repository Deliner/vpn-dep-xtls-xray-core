package deploy

import (
	"context"
	"io"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"vpn-script/internal/control/bundle"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func TestFleetParallelChainsWaitForTheirOwnCleanup(t *testing.T) {
	directory := fleetFixture(t, 2, 2)
	gates := newRemoteGates(t, 4)
	running := startFleet(t, directory, 2, gates, gates.run, io.Discard)
	exits := []int{fleetEvent(t, gates.started), fleetEvent(t, gates.started)}
	slices.Sort(exits)
	if !reflect.DeepEqual(exits, []int{1, 3}) {
		t.Fatalf("expected independent exits first: %v", exits)
	}
	gates.apply[1] <- struct{}{}
	if id := fleetEvent(t, gates.cleaning); id != 1 {
		t.Fatalf("wrong cleanup: %d", id)
	}
	noFleetEvent(t, gates.started)
	gates.cleanup[1] <- struct{}{}
	if id := fleetEvent(t, gates.started); id != 0 {
		t.Fatalf("entry must follow its own exit, without waiting for the other branch: %d", id)
	}
	gates.apply[0] <- struct{}{}
	fleetEvent(t, gates.cleaning)
	gates.cleanup[0] <- struct{}{}
	gates.apply[3] <- struct{}{}
	fleetEvent(t, gates.cleaning)
	gates.cleanup[3] <- struct{}{}
	if id := fleetEvent(t, gates.started); id != 2 {
		t.Fatalf("unexpected remaining entry: %d", id)
	}
	gates.apply[2] <- struct{}{}
	fleetEvent(t, gates.cleaning)
	gates.cleanup[2] <- struct{}{}
	if err := fleetResult(t, running); err != nil {
		t.Fatal(err)
	}
}

func TestFleetConcurrencyBoundAndCompleteResults(t *testing.T) {
	directory := fleetFixture(t, 12, 1)
	var active, peak atomic.Int32
	runner := func(ctx context.Context, _ time.Duration, args ...string) (process.Result, error) {
		if args[len(args)-1] == "uname -m" {
			current := active.Add(1)
			for previous := peak.Load(); current > previous && !peak.CompareAndSwap(previous, current); previous = peak.Load() {
			}
		}
		result, err := successfulRemote(ctx, time.Millisecond, args...)
		if strings.Contains(args[len(args)-1], "'rm' '-rf'") {
			active.Add(-1)
		}
		return result, err
	}
	if err := RunConcurrent(context.Background(), directory, runner, io.Discard, 3); err != nil {
		t.Fatal(err)
	}
	if peak.Load() != 3 || active.Load() != 0 {
		t.Fatalf("unexpected node concurrency: peak=%d active=%d", peak.Load(), active.Load())
	}
	var results map[string]string
	if err := safeio.LoadJSON(filepath.Join(directory, "deploy-result.json"), &results); err != nil {
		t.Fatal(err)
	}
	if len(results) != 12 {
		t.Fatalf("incomplete results: %v", results)
	}
	for id, result := range results {
		if !strings.HasPrefix(result, "applied;") {
			t.Errorf("%s: %s", id, result)
		}
	}
}

func TestDefaultDeploymentPreservesManifestOrder(t *testing.T) {
	directory := fleetFixture(t, 3, 2)
	manifest, err := bundle.Load(directory)
	if err != nil {
		t.Fatal(err)
	}
	var observed []string
	runner := func(ctx context.Context, _ time.Duration, args ...string) (process.Result, error) {
		if args[len(args)-1] == "uname -m" {
			address := strings.TrimPrefix(args[len(args)-2], "root@")
			for id, node := range manifest.Nodes {
				if node.Address == address {
					observed = append(observed, id)
				}
			}
		}
		return successfulRemote(ctx, 0, args...)
	}
	if err := Run(context.Background(), directory, runner, io.Discard); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(observed, manifest.Order) {
		t.Fatalf("default changed application order: got %v want %v", observed, manifest.Order)
	}
}

func TestMultiProtocolNodeWaitsForEveryDownstream(t *testing.T) {
	definition := fleetDefinition(t, 2, 2)
	definition.Nodes["branch00-hop00"].Services["second"] = &model.Service{Protocol: model.Shadowsocks, Port: 24444}
	definition.Routes["route01"].Path[0] = "branch00-hop00.second"
	delete(definition.Nodes, "branch01-hop00")
	directory := renderFleet(t, definition)
	gates := newRemoteGates(t, 4)
	running := startFleet(t, directory, 2, gates, gates.run, io.Discard)
	fleetEvent(t, gates.started)
	fleetEvent(t, gates.started)
	gates.apply[1] <- struct{}{}
	if id := fleetEvent(t, gates.cleaning); id != 1 {
		t.Fatalf("wrong first exit: %d", id)
	}
	gates.cleanup[1] <- struct{}{}
	noFleetEvent(t, gates.started)
	gates.apply[3] <- struct{}{}
	if id := fleetEvent(t, gates.cleaning); id != 3 {
		t.Fatalf("wrong second exit: %d", id)
	}
	gates.cleanup[3] <- struct{}{}
	if id := fleetEvent(t, gates.started); id != 0 {
		t.Fatalf("shared entry did not follow both exits: %d", id)
	}
	gates.apply[0] <- struct{}{}
	fleetEvent(t, gates.cleaning)
	gates.cleanup[0] <- struct{}{}
	if err := fleetResult(t, running); err != nil {
		t.Fatal(err)
	}
}
