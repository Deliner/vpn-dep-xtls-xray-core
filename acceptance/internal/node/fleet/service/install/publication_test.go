package install

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/node/fleet/service"
)

func TestApplyRetainsVersionsAndReloadsIdempotentPublication(t *testing.T) {
	f := newFixture(t)
	first := f.apply(t, "first executable")
	before, err := os.ReadFile(f.tx.store.unit)
	require(t, err)
	repeated := f.apply(t, "first executable")
	if first != repeated || f.reloads != 2 || f.active || f.enabled {
		t.Fatal("idempotent install did not reload or activated the service")
	}
	second := f.apply(t, "updated executable")
	if second.Generation == first.Generation {
		t.Fatal("new binary reused an old generation")
	}
	_, oldUnit, err := f.tx.store.revision(first.Generation)
	require(t, err)
	if !bytes.Equal(before, oldUnit) {
		t.Fatal("old immutable service version changed")
	}
	id, _, err := f.tx.store.current()
	require(t, err)
	if id != second.Generation {
		t.Fatal("new unit did not become the authoritative commit")
	}
}

func TestPublishedUnitNamesPermanentExecutableIndependentlyOfStoreRoot(t *testing.T) {
	f := newFixture(t)
	installed := f.apply(t, "executable")
	_, unit, err := f.tx.store.revision(installed.Generation)
	require(t, err)
	executable := filepath.Join(service.Directory, "releases", installed.Generation, "nhop-fleet")
	if !strings.Contains(string(unit), "ExecStart=:\""+executable+"\"") || strings.Contains(string(unit), f.root) {
		t.Fatalf("unit must reference the permanent published executable: %s", unit)
	}
	require(t, service.PermanentPath(executable))
	if _, err := service.Unit("/tmp/transient/nhop-fleet", "/var/lib/fleet-node", time.Second); err == nil {
		t.Fatal("temporary executable remains forbidden for a real service")
	}
}

func TestReloadFailureRestoresPreviousUnitDespiteCallerCancellation(t *testing.T) {
	f := newFixture(t)
	previous := f.apply(t, "old")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.failReload, f.cancel = 1, cancel
	_, err := f.tx.apply(ctx, Options{State: "/var/lib/fleet-node", Interval: time.Second}, []byte("new"))
	if err == nil || ctx.Err() == nil || f.reloads != 3 {
		t.Fatal("fixture did not exercise failure and independent rollback")
	}
	id, _, err := f.tx.store.current()
	require(t, err)
	if id != previous.Generation || !f.loaded || f.active || f.enabled {
		t.Fatal("rollback lost the previous stopped unit")
	}
}

func TestInterruptedUnitPublicationCanBeReopenedAndReloaded(t *testing.T) {
	f := newFixture(t)
	f.apply(t, "old")
	_, old, err := f.tx.store.current()
	require(t, err)
	options := Options{State: "/var/lib/fleet-node", Interval: time.Second}
	id, err := f.tx.store.publish(options, []byte("new"))
	require(t, err)
	_, next, err := f.tx.store.revision(id)
	require(t, err)
	require(t, f.tx.replace(old, next)) // Interrupted before daemon-reload.
	require(t, f.tx.store.lock.Close())
	f.tx.store, err = openStore(filepath.Join(f.root, "service"), filepath.Join(f.root, "agent.service"), false)
	require(t, err)
	receipt := f.apply(t, "new")
	if receipt.Generation != id || f.reloads != 2 {
		t.Fatal("reopened unit was not reconciled with systemd")
	}
}

func TestControlTargetsOnlyAgentAndRemovalRetainsVersions(t *testing.T) {
	f := newFixture(t)
	receipt := f.apply(t, "binary")
	require(t, f.tx.activate(context.Background(), "start"))
	if !f.active || !f.enabled {
		t.Fatal("start did not enable the persistent agent")
	}
	if err := f.tx.inactive(context.Background(), "present"); err == nil {
		t.Fatal("active enabled unit accepted for replacement")
	}
	require(t, f.tx.activate(context.Background(), "stop"))
	_, previous, err := f.tx.store.current()
	require(t, err)
	require(t, f.tx.publish(context.Background(), previous, nil))
	if _, err := os.Lstat(f.tx.store.unit); !os.IsNotExist(err) || f.loaded {
		t.Fatal("removed unit remains present")
	}
	_, _, err = f.tx.store.revision(receipt.Generation)
	require(t, err)
}

func TestStoppedFailedAgentCanBeRemovedWithoutResettingOtherUnits(t *testing.T) {
	f := newFixture(t)
	f.apply(t, "binary")
	f.enabled = true
	f.override["ActiveState"], f.override["SubState"] = "failed", "failed"
	f.tx.manager = func(ctx context.Context, args ...string) ([]byte, error) {
		if args[0] == "reset-failed" && strings.Join(args, " ") != "reset-failed -- "+service.Name {
			t.Fatal("failure reset did not target only the owned agent")
		}
		return f.manage(ctx, args...)
	}
	require(t, f.tx.activate(context.Background(), "stop"))
	if f.resets != 1 || f.enabled || f.active {
		t.Fatal("failed service was not stopped, disabled and reset")
	}
	_, previous, err := f.tx.store.current()
	require(t, err)
	require(t, f.tx.publish(context.Background(), previous, nil))
	if f.loaded {
		t.Fatal("reset failed service could not be unregistered")
	}
}
