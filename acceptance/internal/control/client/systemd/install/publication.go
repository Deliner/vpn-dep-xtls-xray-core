package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	systemmanager "vpn-script/internal/control/client/systemd/manager"
	"vpn-script/internal/platform/safeio"
)

func (t transaction) apply(ctx context.Context, b bundle) (Receipt, error) {
	if err := t.store.manual(); err != nil {
		return Receipt{}, err
	}
	previous, err := t.store.current()
	if err != nil {
		return Receipt{}, err
	}
	if err := t.inactive(ctx, "either"); err != nil {
		return Receipt{}, err
	}
	r, unit, err := t.store.stage(b)
	if err != nil {
		return Receipt{}, err
	}
	return r, t.publish(ctx, previous, unit)
}

func (t transaction) remove(ctx context.Context) error {
	if err := t.store.manual(); err != nil {
		return err
	}
	previous, err := t.store.current()
	if err != nil {
		return err
	}
	if err := t.inactive(ctx, "either"); err != nil {
		return err
	}
	return t.publish(ctx, previous, nil)
}

// The unit is the single durable commit point. Old immutable generations stay
// valid if the process dies before daemon-reload. Retrying always reloads, even
// when unit bytes already match, so no second mutable "current" file is needed.
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
		recovery, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		rollback := t.replace(desired, previous)
		if rollback == nil {
			rollback = t.reload(recovery, previous != nil)
		}
		if rollback != nil {
			return errors.Join(fmt.Errorf("client unit reload/verification failed: %w", err), fmt.Errorf("rollback failed; inspect retained generations: %w", rollback))
		}
		return fmt.Errorf("client unit reload/verification failed; previous unit restored: %w", err)
	}
	return nil
}

func (t transaction) replace(expected, desired []byte) error {
	actual, err := t.store.current()
	if err != nil {
		return err
	}
	if !bytes.Equal(actual, expected) {
		return fmt.Errorf("client unit changed during installation; refusing replacement")
	}
	if desired != nil {
		if expected == nil {
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

func (t transaction) inactive(ctx context.Context, expected string) error {
	data, commandErr := t.manager(ctx, "show", "--property="+properties, "--", Service)
	state, err := systemmanager.Parse(data, properties)
	if err != nil {
		return fmt.Errorf("cannot inspect client systemd state: %w", err)
	}
	if state["ActiveState"] != "inactive" || state["SubState"] != "dead" || state["DropInPaths"] != "" || state["Job"] != "" {
		return fmt.Errorf("client unit is active, queued or overridden; explicitly disconnect and stop it before installation")
	}
	if state["LoadState"] == "not-found" && expected != "present" && state["FragmentPath"] == "" && state["UnitFileState"] == "" {
		return nil
	}
	if commandErr != nil || expected == "absent" || state["LoadState"] != "loaded" || state["FragmentPath"] != t.store.unit || state["UnitFileState"] != "static" {
		return fmt.Errorf("refusing unknown, enabled, failed or foreign systemd client unit")
	}
	return nil
}
