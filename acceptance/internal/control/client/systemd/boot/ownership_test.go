package boot

import (
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestCompletedRegistrationNeverRepairsMissingOrChangedArtifacts(t *testing.T) {
	for _, corruption := range []string{"missing", "bytes", "link", "mode"} {
		t.Run(corruption, func(t *testing.T) {
			tr, _ := fixture(t)
			_, err := tr.enable(t.Context(), "")
			require(t, err)
			value, _, err := tr.read()
			require(t, err)
			files, err := tr.artifacts(value)
			require(t, err)
			corrupt(t, files, corruption)
			before, err := os.ReadFile(tr.state)
			require(t, err)
			if _, err := tr.enable(t.Context(), ""); err == nil {
				t.Fatal("unknown registration was repaired")
			}
			if _, err := tr.disable(t.Context()); err == nil {
				t.Fatal("unknown registration was removed")
			}
			after, err := os.ReadFile(tr.state)
			require(t, err)
			if string(before) != string(after) {
				t.Fatal("refused operation changed the recorded ownership")
			}
		})
	}
}

func corrupt(t *testing.T, files []artifact, kind string) {
	t.Helper()
	switch kind {
	case "missing":
		require(t, os.Remove(files[0].path))
	case "bytes":
		require(t, safeio.Atomic(files[1].path, []byte("foreign"), 0644))
	case "link":
		require(t, os.Remove(files[len(files)-1].path))
		require(t, os.Symlink("/etc/passwd", files[len(files)-1].path))
	case "mode":
		require(t, os.Chmod(files[2].path, 0666))
	}
}

func TestUnrecordedIdenticalArtifactsAreNotAdopted(t *testing.T) {
	tr, _ := fixture(t)
	value := record{OwnerToken: tr.snapshot.OwnerToken, Generation: tr.snapshot.Generation, NetworkManager: "systemd-networkd.service"}
	files, err := tr.artifacts(value)
	require(t, err)
	require(t, tr.create(files[1]))
	if _, err := tr.enable(t.Context(), ""); err == nil {
		t.Fatal("unrecorded network drop-in adopted")
	}
	if _, err := tr.disable(t.Context()); err == nil {
		t.Fatal("unrecorded network drop-in removed")
	}
	if _, err := os.Lstat(tr.state); !os.IsNotExist(err) {
		t.Fatal("refused operation created a journal", err)
	}
}

func TestForeignStateAndLinkedParentsAreRejectedWithoutDeletion(t *testing.T) {
	for _, scenario := range []string{"state", "parent"} {
		t.Run(scenario, func(t *testing.T) {
			tr, _ := fixture(t)
			path := tr.state
			if scenario == "state" {
				require(t, safeio.NewJSON(path, record{SchemaVersion: 2}))
			} else {
				path = filepath.Join(tr.units, "systemd-networkd.service.d")
				require(t, os.Symlink(t.TempDir(), path))
			}
			if _, err := tr.enable(t.Context(), ""); err == nil {
				t.Fatal("foreign state accepted")
			}
			if _, err := tr.disable(t.Context()); err == nil {
				t.Fatal("foreign state deleted")
			}
			if _, err := os.Lstat(path); err != nil {
				t.Fatal("foreign path was removed", err)
			}
		})
	}
}

func TestDisableCompletesEveryInterruptedPublication(t *testing.T) {
	for boundary := 0; boundary <= 5; boundary++ {
		t.Run(string(rune('0'+boundary)), func(t *testing.T) {
			tr, _ := fixture(t)
			value, err := tr.begin(t.Context(), "")
			require(t, err)
			files, err := tr.artifacts(value)
			require(t, err)
			for _, item := range files[:boundary] {
				require(t, tr.create(item))
			}
			_, err = tr.disable(t.Context())
			require(t, err)
			if _, found, err := tr.read(); err != nil || found {
				t.Fatal("explicit disable retained registration", err)
			}
			require(t, tr.unregistered(t.Context()))
		})
	}
}
