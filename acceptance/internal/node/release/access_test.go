package release

import (
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

func accessFixture(t *testing.T) (string, model.NodeConfig, map[string]string) {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0750); err != nil {
		t.Fatal(err)
	}
	data := model.NodeConfig{SchemaVersion: 2, Programs: []model.Program{{Name: "xray"}, {Name: "sing-box"}}}
	for _, name := range names(data) {
		mode, err := fileMode(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := safeio.Atomic(filepath.Join(directory, name), []byte("fixture "+name), mode); err != nil {
			t.Fatal(err)
		}
	}
	files, err := Inventory(directory, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(directory, files, os.Geteuid(), os.Getegid()); err != nil {
		t.Fatal(err)
	}
	return directory, data, files
}

func TestReleasePermissionsCoverEveryExecutableAndConfiguration(t *testing.T) {
	for _, name := range []string{"xray", "sing-box", "nhop-node", "libcronet.so", "xray.json", "sing-box.json", "processes.json"} {
		t.Run(name, func(t *testing.T) {
			directory, data, files := accessFixture(t)
			path := filepath.Join(directory, name)
			if err := os.Chmod(path, 0666); err != nil {
				t.Fatal(err)
			}
			if err := Permissions(directory, data, os.Geteuid(), os.Getegid()); err == nil {
				t.Fatal("candidate with writable files accepted")
			}
			if err := Verify(directory, files, os.Geteuid(), os.Getegid()); err == nil {
				t.Fatal("committed writable file accepted")
			}
		})
	}
}

func TestCommittedInventoryRejectsUnknownMissingExtraAndChangedContents(t *testing.T) {
	for _, change := range []string{"unknown-name", "missing", "extra", "changed", "empty-inventory"} {
		t.Run(change, func(t *testing.T) {
			directory, _, files := accessFixture(t)
			var err error
			switch change {
			case "unknown-name":
				files["../outside"] = files["xray"]
			case "missing":
				err = os.Remove(filepath.Join(directory, "xray.json"))
			case "extra":
				err = safeio.Atomic(filepath.Join(directory, "unexpected"), []byte("extra"), 0600)
			case "changed":
				err = os.WriteFile(filepath.Join(directory, "xray"), []byte("modified"), 0755)
			case "empty-inventory":
				files = nil
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := Verify(directory, files, os.Geteuid(), os.Getegid()); err == nil {
				t.Fatal("untrusted rollback inventory accepted")
			}
		})
	}
}
