package install

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestForeignUnitAndModifiedVersionsAreNeverReplaced(t *testing.T) {
	for _, file := range []string{"unit", "nhop-fleet", "unit.txt", "revision.json"} {
		t.Run(file, func(t *testing.T) {
			f := newFixture(t)
			receipt := f.apply(t, "old")
			path := filepath.Join(f.tx.store.base, "releases", receipt.Generation, file)
			if file == "unit" {
				path = f.tx.store.unit
			}
			require(t, os.WriteFile(path, []byte("foreign contents"), 0600))
			_, err := f.tx.apply(context.Background(), Options{State: "/var/lib/fleet-node", Interval: time.Second}, []byte("new"))
			if err == nil || f.reloads != 1 {
				t.Fatal("unverified installation was replaced or reloaded")
			}
			raw, err := os.ReadFile(path)
			require(t, err)
			if string(raw) != "foreign contents" {
				t.Fatal("foreign bytes were repaired or deleted")
			}
		})
	}
}

func TestEnabledOverriddenAndForeignManagerStateBlockInstallation(t *testing.T) {
	for key, value := range map[string]string{"UnitFileState": "enabled", "DropInPaths": "/foreign/override.conf", "FragmentPath": "/other/agent.service", "Job": "12", "ActiveState": "active"} {
		t.Run(key, func(t *testing.T) {
			f := newFixture(t)
			before := f.apply(t, "old")
			f.override[key] = value
			_, err := f.tx.apply(context.Background(), Options{State: "/var/lib/fleet-node", Interval: time.Second}, []byte("new"))
			if err == nil || f.reloads != 1 {
				t.Fatal("unsafe system manager state accepted")
			}
			id, _, err := f.tx.store.current()
			require(t, err)
			if id != before.Generation {
				t.Fatal("rejected operation changed the current unit")
			}
		})
	}
}

func TestMissingOwnerAndConcurrentInstallerAreRejected(t *testing.T) {
	f := newFixture(t)
	if other, err := openStore(f.tx.store.base, f.tx.store.unit, false); err == nil {
		other.lock.Close()
		t.Fatal("second service installer acquired the same state")
	}
	path := filepath.Join(f.tx.store.base, "owner.json")
	require(t, os.Remove(path))
	if _, err := openStore(f.tx.store.base, f.tx.store.unit, true); err == nil {
		t.Fatal("missing ownership was reset")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("missing owner was silently recreated")
	}
}

func TestServiceManagementRequiresLinuxRootBeforeFileAccess(t *testing.T) {
	if runtime.GOOS == "linux" && os.Geteuid() == 0 && os.Getegid() == 0 {
		t.Skip("non-Linux/non-root refusal contract")
	}
	_, err := Apply(context.Background(), Options{State: "missing"})
	if err == nil || !strings.Contains(err.Error(), "Linux root") {
		t.Fatalf("platform guard did not run first: %v", err)
	}
	for _, action := range []string{"start", "stop", "remove"} {
		if err := Control(context.Background(), action); err == nil || !strings.Contains(err.Error(), "Linux root") {
			t.Fatalf("control platform guard failed: %v", err)
		}
	}
}
