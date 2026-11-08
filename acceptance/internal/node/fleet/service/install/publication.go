package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"vpn-script/internal/platform/safeio"
)

func (t transaction) publish(ctx context.Context, previous, desired []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !bytes.Equal(previous, desired) {
		if err := t.replace(previous, desired); err != nil {
			return err
		}
	}
	if err := t.reload(ctx, desired != nil); err != nil {
		recovery, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		rollback := t.replace(desired, previous)
		if rollback == nil {
			rollback = t.reload(recovery, previous != nil)
		}
		if rollback != nil {
			return errors.Join(fmt.Errorf("Fleet unit reload failed: %w", err), fmt.Errorf("rollback failed; inspect retained generations: %w", rollback))
		}
		return fmt.Errorf("Fleet unit reload failed; previous unit restored: %w", err)
	}
	return nil
}

func (t transaction) replace(expected, desired []byte) error {
	_, actual, err := t.store.current()
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("Fleet unit changed during installation; refusing replacement")
	}
	if desired != nil {
		if actual == nil {
			return safeio.Create(t.store.unit, desired, 0644)
		}
		return safeio.Atomic(t.store.unit, desired, 0644)
	}
	if actual == nil {
		return nil
	}
	if err := os.Remove(t.store.unit); err != nil {
		return err
	}
	return safeio.SyncDir(filepath.Dir(t.store.unit))
}

func (t transaction) reload(ctx context.Context, installed bool) error {
	if _, err := t.manager(ctx, "daemon-reload"); err != nil {
		return fmt.Errorf("systemd daemon-reload failed: %w", err)
	}
	expected := "absent"
	if installed {
		expected = "present"
	}
	return t.inactive(ctx, expected)
}
