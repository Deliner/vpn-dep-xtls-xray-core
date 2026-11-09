package release

import (
	"os"
	"path/filepath"
	"testing"
	"vpn-script/internal/platform/safeio"
)

func TestReviewedInventoryRejectsSubstitutionAndUnrecordedInputs(t *testing.T) {
	for _, damage := range []string{"", "changed", "missing", "extra", "linked", "manifest", "nested", "missing-nested"} {
		t.Run(damage, func(t *testing.T) {
			root := t.TempDir()
			put(t, root, "value", []byte("tested"))
			put(t, root, "nested/evidence.sha256", []byte("reviewed nested manifest"))
			manifest := []byte(safeio.Hash([]byte("tested")) + "  value\n")
			put(t, root, "evidence.sha256", manifest)
			extras := map[string]string{"nested/evidence.sha256": safeio.Hash([]byte("reviewed nested manifest"))}
			switch damage {
			case "changed", "extra", "manifest":
				name := map[string]string{"changed": "value", "extra": "unrecorded", "manifest": "evidence.sha256"}[damage]
				put(t, root, name, []byte("other"))
			case "missing", "linked":
				if err := os.Remove(filepath.Join(root, "value")); err != nil {
					t.Fatal(err)
				}
				if damage == "linked" {
					if err := os.Symlink("nested/evidence.sha256", filepath.Join(root, "value")); err != nil {
						t.Fatal(err)
					}
				}
			case "missing-nested":
				if err := os.Remove(filepath.Join(root, "nested/evidence.sha256")); err != nil {
					t.Fatal(err)
				}
			case "nested":
				put(t, root, "nested/evidence.sha256", []byte("other"))
			}
			_, err := inventory(root, safeio.Hash(manifest), extras)
			if (err == nil) != (damage == "") {
				t.Fatalf("damage %q: %v", damage, err)
			}
		})
	}
}

func put(t *testing.T, root, name string, data []byte) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
