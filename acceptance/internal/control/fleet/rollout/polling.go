package rollout

import (
	"context"
	"time"

	"vpn-script/internal/control/fleet/rollout/journal"
)

// Tick publishes at most sixteen nodes per active plan. Every iteration checks
// all previously published stages before any further assignments are released.
func (m *Manager) Tick(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.available(); err != nil {
		return err
	}
	names, err := m.journal.Names()
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return err
		}
		record, err := m.journal.Read(name)
		if err != nil {
			return err
		}
		if record.Phase == "running" {
			if err := m.advance(ctx, &record); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) advance(ctx context.Context, record *journal.Record) error {
	now := m.now().Unix()
	if now-record.StageStartedAt >= record.Spec.StageTimeoutSeconds {
		fail(record, "stage_timeout", "")
	} else if err := m.observe(record, now); err != nil {
		return err
	}
	if err := m.commit(record, now); err != nil || record.Phase != "running" {
		return err
	}
	if !stagesReady(*record, record.Stage-1) {
		return nil
	}
	if stagesReady(*record, record.Stage) {
		if record.Stage == len(record.Spec.Stages)-1 {
			record.Phase = "succeeded"
		} else {
			record.Stage++
			record.StageStartedAt = max(record.UpdatedAt, now)
		}
		return m.commit(record, now)
	}
	return m.publishStage(ctx, record, now)
}

func (m *Manager) publishStage(ctx context.Context, record *journal.Record, now int64) error {
	published := 0
	for index, target := range record.Targets[record.Stage] {
		if err := ctx.Err(); err != nil {
			return err
		}
		if target.PublishedRevision != 0 {
			continue
		}
		if err := m.publish(record, index, now); err != nil || record.Phase != "running" {
			return err
		}
		published++
		if published == 16 {
			break
		}
	}
	return nil
}

func (m *Manager) Run(ctx context.Context) error {
	for {
		if err := m.Tick(ctx); err != nil {
			return err
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
