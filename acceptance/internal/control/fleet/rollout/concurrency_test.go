package rollout

import (
	"context"
	"errors"
	"sync"
	"testing"

	"vpn-script/internal/control/fleet/registry"
)

func TestConcurrentPublishersCannotCreateOverlappingRollouts(t *testing.T) {
	f := newFixture(t)
	results := make(chan error, 2)
	var done sync.WaitGroup
	for _, id := range []string{"release-a", "release-b"} {
		done.Add(1)
		go func(id string) {
			defer done.Done()
			spec := f.spec
			spec.ID = id
			_, err := f.m.Create(spec)
			results <- err
		}(id)
	}
	done.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, registry.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 || len(f.b.calls) != 0 {
		t.Fatal("overlapping rollout creation was not serialized before publication")
	}
}

func TestCanceledWorkerNeverPublishesThePreparedIntent(t *testing.T) {
	f := newFixture(t)
	_, err := f.m.Create(f.spec)
	require(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(f.m.Run(ctx), context.Canceled) || len(f.b.calls) != 0 {
		t.Fatal("canceled coordinator released a node assignment")
	}
}
