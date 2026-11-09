package installation

import (
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestUpgradeMustRetainActualOldReleaseAndHistory(t *testing.T) {
	for _, damage := range []string{"", "same-release", "previous-id", "previous-generation", "supervisor", "old-contents", "old-extra"} {
		t.Run(damage, func(t *testing.T) {
			before, after, old, sha := upgradeFixture(t)
			switch damage {
			case "same-release":
				after.Directory = before.Directory
			case "previous-id":
				after.State.PreviousReleaseID = "foreign"
			case "previous-generation":
				after.State.PreviousGeneration = "new-generation"
			case "supervisor":
				after.State.Files["nhop-node"] = before.State.Files["nhop-node"]
			case "old-contents", "old-extra":
				name := "xray"
				if damage == "old-extra" {
					name = "unexpected"
				}
				if err := os.WriteFile(filepath.Join(before.Directory, name), []byte("changed"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := VerifyUpgrade(before, after, old, sha); (err == nil) != (damage == "") {
				t.Fatalf("upgrade evidence mismatch: %v", err)
			}
		})
	}
}

func TestRetainedRuntimeIdentityDetectsLaterMutation(t *testing.T) {
	previous, _, directory := runtimeFixture(t)
	alias := filepath.Join(filepath.Dir(directory), "current")
	if err := os.Symlink(directory, alias); err != nil {
		t.Fatal(err)
	}
	for role, value := range previous {
		value.Path = filepath.Join(alias, filepath.Base(value.Path))
		previous[role] = value
	}
	if err := RetainRuntimes(previous, directory); err != nil {
		t.Fatal(err)
	}
	for _, value := range previous {
		if !value.VerifiedUnchanged || filepath.Dir(value.Path) != directory {
			t.Fatal("old interval not bound to retained release")
		}
	}
	if err := os.Remove(alias); err != nil {
		t.Fatal(err)
	}
	if err := RetainRuntimes(previous, directory); err != nil {
		t.Fatal("retained identities still depended on current symlink", err)
	}
	if err := os.WriteFile(filepath.Join(directory, "xray"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := RetainRuntimes(previous, directory); err == nil {
		t.Fatal("retained executable mutation accepted")
	}
}

func TestRuntimeUpgradeRequiresDifferentCompleteExecutableIdentities(t *testing.T) {
	for _, damage := range []string{"", "missing", "unchanged", "error", "invalid", "foreign-role"} {
		t.Run(damage, func(t *testing.T) {
			old, current, _ := runtimeFixture(t)
			switch damage {
			case "missing":
				delete(current, "client/xray")
			case "unchanged":
				current = old
			case "error", "invalid":
				value := current["client/xray"]
				if damage == "error" {
					value.Error = "failed capture"
				} else {
					value.SHA256 = "invalid"
				}
				current["client/xray"] = value
			case "foreign-role":
				current["foreign"] = current["client/xray"]
				delete(current, "client/xray")
			}
			if err := RuntimeTransition(old, current); (err == nil) != (damage == "") {
				t.Fatalf("runtime transition mismatch: %v", err)
			}
		})
	}
}

func TestSupervisorUpgradeCannotReuseCurrentExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node")
	data := []byte("current-supervisor")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, previous := range []string{"", safeio.Hash(data)} {
		if _, err := UpgradeSupervisor(previous, path); err == nil {
			t.Fatal("same or missing previous installer identity accepted")
		}
	}
	if _, err := UpgradeSupervisor(safeio.Hash([]byte("old")), path); err != nil {
		t.Fatal(err)
	}
}
