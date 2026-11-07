package catalog

import (
	"context"
	"errors"
	"testing"
)

func TestBatchRejectsInvalidBoundsBeforeGitHub(t *testing.T) {
	run := func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("invalid batch contacted GitHub")
		return nil, nil
	}
	for _, limits := range [][2]int{{-1, 1}, {0, 0}, {0, 51}} {
		if _, err := Apply(context.Background(), run, limits[0], limits[1], "Deliner"); err == nil {
			t.Fatal(limits)
		}
	}
	if _, err := Apply(context.Background(), run, 0, 1, "different"); err == nil {
		t.Fatal("accepted different owner")
	}
}

func TestCancelledBatchDoesNotCreateRepositories(t *testing.T) {
	fixture(t)
	if _, err := Build(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run := func(context.Context, string, ...string) ([]byte, error) {
		t.Fatal("cancelled batch contacted GitHub")
		return nil, nil
	}
	batch, err := Apply(ctx, run, 0, 1, "Deliner")
	if !errors.Is(err, context.Canceled) || len(batch.Verified) != 0 || batch.Failure == "" {
		t.Fatalf("lost cancellation result: %+v, %v", batch, err)
	}
}
