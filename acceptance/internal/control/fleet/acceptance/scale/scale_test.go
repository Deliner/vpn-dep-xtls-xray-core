package scale

import (
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
)

func TestFleetHTTPSScale(t *testing.T) {
	if os.Getenv("NHOP_FLEET_SCALE_NODES") == "" {
		t.Skip("explicit capacity acceptance: just fleet-scale-test")
	}
	nodes, err := strconv.Atoi(os.Getenv("NHOP_FLEET_SCALE_NODES"))
	require(t, err)
	workers, err := strconv.Atoi(os.Getenv("NHOP_FLEET_SCALE_WORKERS"))
	require(t, err)
	if nodes < 1 || nodes > 10000 || workers < 1 || workers > 8 {
		t.Fatal("capacity acceptance requires 1..10000 nodes and 1..8 workers")
	}
	f := newFixture(t)
	started := time.Now()
	expected := f.populate(t, nodes, workers)
	f.verify(t, expected)
	f.revoke(t, expected)
	f.restart(t, expected)
	t.Logf("Fleet HTTPS capacity PASS: nodes=%d workers=%d revoked=%d elapsed=%s; control plane only, no node installers", nodes, workers, (nodes+9)/10, time.Since(started))
}

func (f *fixture) populate(t *testing.T, nodes, workers int) []wire.Node {
	t.Helper()
	jobs := make(chan int, nodes)
	for index := range nodes {
		jobs <- index
	}
	close(jobs)
	values := make([]wire.Node, nodes)
	var group sync.WaitGroup
	var completed atomic.Int32
	for range workers {
		group.Go(func() {
			for index := range jobs {
				value, err := f.exercise(t.Context(), fmt.Sprintf("node-%05d", index))
				if err != nil {
					t.Errorf("node %d: %v", index, err)
					return
				}
				values[index] = value
				completed.Add(1)
			}
		})
	}
	group.Wait()
	if t.Failed() || int(completed.Load()) != nodes {
		t.Fatalf("incomplete capacity lifecycle: completed %d/%d", completed.Load(), nodes)
	}
	return values
}
