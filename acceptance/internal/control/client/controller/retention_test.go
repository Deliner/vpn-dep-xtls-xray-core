package controller

import (
	"os"
	"path/filepath"
	"testing"
)

func TestForeignSnapshotMembersAreNeverPruned(t *testing.T) {
	for _, kind := range []string{"foreign", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "native")
			var err error
			switch kind {
			case "foreign":
				err = os.WriteFile(filepath.Join(directory, "unmanaged"), []byte("keep"), 0600)
			case "directory":
				err = os.Mkdir(path, 0700)
			case "symlink":
				err = os.Symlink("/nonexistent", path)
			}
			if err != nil {
				t.Fatal(err)
			}
			if pruneStopped(directory) == nil {
				t.Fatal("unknown snapshot state accepted")
			}
			if items, _ := os.ReadDir(directory); len(items) != 1 {
				t.Fatal("refused cleanup altered existing state")
			}
		})
	}
}
