package cleanup

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"vpn-script/internal/quality/dependencies/commands"
	"vpn-script/internal/quality/dependencies/records"
	"vpn-script/internal/quality/dependencies/scope"
)

func loadPlan(expectedSHA string) (Plan, error) {
	var plan Plan
	digest, err := records.Read("cleanup-plan.json", &plan)
	if err != nil {
		return plan, err
	}
	keep, scopeSHA, err := scope.Read()
	if err != nil {
		return plan, err
	}
	if digest != expectedSHA || plan.Schema != 1 || plan.AccountID != accountID || plan.ScopeSHA != scopeSHA || !reflect.DeepEqual(keep, plan.Keep) || len(plan.Unknown) != 0 {
		return plan, fmt.Errorf("cleanup plan, protected scope or recorded ownership changed")
	}
	return plan, nil
}

func Apply(ctx context.Context, run commands.Runner, digest string, offset, limit int) (Batch, error) {
	batch := Batch{PlanSHA: digest, Offset: offset, Verified: []string{}}
	if offset < 0 || limit < 1 || limit > 50 {
		return batch, fmt.Errorf("expected nonnegative offset and cleanup batch size 1..50")
	}
	plan, err := loadPlan(digest)
	if err != nil {
		return batch, err
	}
	if offset >= len(plan.Delete) {
		return batch, fmt.Errorf("offset exceeds %d cleanup targets", len(plan.Delete))
	}
	if err := authenticate(ctx, run); err != nil {
		return batch, err
	}
	name := fmt.Sprintf("cleanup-batch-%04d.json", offset)
	for _, target := range plan.Delete[offset:min(offset+limit, len(plan.Delete))] {
		if err := pacedRemoval(ctx, run, target, plan.Keep); err != nil {
			batch.Failure = err.Error()
			if saveErr := records.Save(name, batch); saveErr != nil {
				return batch, fmt.Errorf("removal: %v; checkpoint: %w", err, saveErr)
			}
			return batch, err
		}
		batch.Verified = append(batch.Verified, target.Name)
		if err := records.Save(name, batch); err != nil {
			return batch, err
		}
	}
	return batch, nil
}

func pacedRemoval(ctx context.Context, run commands.Runner, target Target, keep []scope.Fork) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	return remove(ctx, run, target, keep)
}
