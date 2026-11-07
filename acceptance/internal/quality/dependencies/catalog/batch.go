package catalog

import (
	"context"
	"fmt"
	"time"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/github"
)

func Apply(ctx context.Context, run commands.Runner, offset, limit int, owner string) (Batch, error) {
	return apply(ctx, run, offset, limit, owner, false)
}

func ApplyAdditional(ctx context.Context, run commands.Runner, offset, limit int, owner string) (Batch, error) {
	return apply(ctx, run, offset, limit, owner, true)
}

func apply(ctx context.Context, run commands.Runner, offset, limit int, owner string, additional bool) (Batch, error) {
	if offset < 0 || limit < 1 || limit > 50 || owner != "Deliner" {
		return Batch{}, fmt.Errorf("expected nonnegative offset, batch size 1..50 and owner Deliner")
	}
	plan, digest, err := loadPlan()
	prefix := "batch"
	if additional {
		plan, digest, err = loadAdditionalPlan()
		prefix = "additional-batch"
	}
	if err != nil {
		return Batch{}, err
	}
	if offset >= len(plan.Entries) {
		return Batch{}, fmt.Errorf("offset exceeds %d planned repositories", len(plan.Entries))
	}
	batch := Batch{PlanSHA: digest, Offset: offset, Limit: limit, Verified: []string{}}
	path := fmt.Sprintf("build/dependencies/%s-%04d.json", prefix, offset)
	for _, entry := range plan.Entries[offset:min(offset+limit, len(plan.Entries))] {
		if err := pacedFork(ctx, run, entry.Upstream, owner); err != nil {
			batch.Failure = err.Error()
			return batch, recordFailure(path, batch, err)
		}
		batch.Verified = append(batch.Verified, entry.Upstream)
		if err := safeio.JSON(path, batch); err != nil {
			return batch, err
		}
	}
	return batch, nil
}

func pacedFork(ctx context.Context, run commands.Runner, upstream, owner string) error {
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	_, err := github.Fork(ctx, run, upstream, owner)
	return err
}

func recordFailure(path string, batch Batch, cause error) error {
	if err := safeio.JSON(path, batch); err != nil {
		return fmt.Errorf("fork failed: %v; recording failure: %w", cause, err)
	}
	return cause
}
