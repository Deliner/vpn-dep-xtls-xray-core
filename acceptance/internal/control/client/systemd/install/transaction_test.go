package install

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestInstallationSnapshotsInputsAndRetainsExplicitRollback(t *testing.T) {
	tr, manager, b := installation(t)
	first := requireApply(t, tr, b)
	original := readUnit(t, tr)
	if strings.Contains(string(original), "[Install]") || strings.Contains(string(original), "ExecStop=") {
		t.Fatal("installation advertises unsupported boot/disconnect behavior")
	}
	if repeated := requireApply(t, tr, b); repeated != first || !bytes.Equal(readUnit(t, tr), original) {
		t.Fatal("unchanged desired inputs created a different generation")
	}
	changed := b
	changed.identity.Network.MTU = 1400
	second := requireApply(t, tr, changed)
	if first.Generation == second.Generation {
		t.Fatal("changed network intent reused a generation")
	}
	restored := requireApply(t, tr, b)
	if restored != first || !bytes.Equal(readUnit(t, tr), original) {
		t.Fatal("explicit rollback did not restore the exact prior unit")
	}
	if err := tr.remove(context.Background()); err != nil || manager.loaded != "" {
		t.Fatal("unit was not unregistered", err)
	}
	for _, id := range []string{first.Generation, second.Generation} {
		if _, err := tr.store.generation(id); err != nil {
			t.Fatal("unregistering removed recovery inputs", err)
		}
	}
	for _, call := range manager.calls {
		if !strings.HasPrefix(call, "show ") && call != "daemon-reload" {
			t.Fatal("installer controlled service activation", call)
		}
	}
}

func TestReloadFailureRollsBackInitialAndExistingUnits(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "first-install", true: "update"}[existing], func(t *testing.T) {
			tr, manager, b := installation(t)
			var before []byte
			if existing {
				requireApply(t, tr, b)
				before = readUnit(t, tr)
			}
			b.identity.Network.MTU = 1400
			manager.failReloads = 1
			if _, err := tr.apply(context.Background(), b); err == nil {
				t.Fatal("reload failure reported success")
			}
			after, err := tr.store.current()
			if err != nil || !bytes.Equal(before, after) || (manager.loaded != "") != existing {
				t.Fatal("reload failure changed the previous installation", err)
			}
			requireApply(t, tr, b)
		})
	}
}

func TestInterruptedPublicationReconcilesTheCommittedUnit(t *testing.T) {
	tr, manager, b := installation(t)
	first := requireApply(t, tr, b)
	previous := readUnit(t, tr)
	b.identity.Network.MTU = 1400
	second, unit, err := tr.store.stage(b)
	if err != nil || tr.replace(previous, unit) != nil {
		t.Fatal("cannot simulate publication before manager reload", err)
	}
	count := len(manager.calls)
	if retried := requireApply(t, tr, b); retried != second || len(manager.calls) <= count+1 {
		t.Fatal("retry failed to reconcile an already published unit")
	}
	if _, err := tr.store.generation(first.Generation); err != nil {
		t.Fatal("interruption destroyed the old manager's executable snapshot", err)
	}
}

func TestUnregisterFailureRestoresTheSameOwnedUnit(t *testing.T) {
	tr, manager, b := installation(t)
	requireApply(t, tr, b)
	before := readUnit(t, tr)
	manager.failReloads = 1
	if err := tr.remove(context.Background()); err == nil {
		t.Fatal("unregister reload failure was accepted")
	}
	if !bytes.Equal(before, readUnit(t, tr)) || manager.loaded != tr.store.unit {
		t.Fatal("failed unregister lost the previous unit")
	}
}

func TestCancellationAndFailedRollbackDoNotPretendSuccess(t *testing.T) {
	tr, manager, b := installation(t)
	requireApply(t, tr, b)
	before := readUnit(t, tr)
	b.identity.Network.MTU = 1400
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := tr.apply(ctx, b); err == nil || !bytes.Equal(before, readUnit(t, tr)) {
		t.Fatal("canceled install changed the unit or reported success")
	}
	manager.failReloads = 2
	if _, err := tr.apply(context.Background(), b); err == nil || !bytes.Equal(before, readUnit(t, tr)) {
		t.Fatal("failed reload/rollback did not retain the old disk unit with an error")
	}
	requireApply(t, tr, b)
}

func TestForeignUnitAndModifiedGenerationAreNeverOverwritten(t *testing.T) {
	for _, member := range []string{"unit", "xray", "runtimes.json", "extra", "mode", "symlink"} {
		t.Run(member, func(t *testing.T) {
			tr, _, b := installation(t)
			r := requireApply(t, tr, b)
			path := filepath.Join(tr.store.base, "releases", r.Generation, member)
			if member == "unit" {
				path = tr.store.unit
			}
			corruptGeneration(t, path, member)
			before, _ := os.ReadFile(tr.store.unit)
			if _, err := tr.apply(context.Background(), b); err == nil {
				t.Fatal("changed managed data was adopted")
			}
			if err := tr.remove(context.Background()); err == nil {
				t.Fatal("changed managed data permitted unregister")
			}
			after, _ := os.ReadFile(tr.store.unit)
			if !bytes.Equal(before, after) {
				t.Fatal("refused install/unregister changed the unit")
			}
		})
	}
}

func corruptGeneration(t *testing.T, path, kind string) {
	t.Helper()
	var err error
	switch kind {
	case "mode":
		err = os.Chmod(filepath.Join(filepath.Dir(path), "profile.json"), 0644)
	case "symlink":
		path = filepath.Join(filepath.Dir(path), "xray")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		err = os.Symlink("/bin/true", path)
	default:
		mode := os.FileMode(0600)
		if kind == "unit" {
			mode = 0644
		} else if kind == "xray" {
			mode = 0755
		}
		err = safeio.Atomic(path, []byte("foreign bytes\n"), mode)
	}
	if err != nil {
		t.Fatal(err)
	}
}
