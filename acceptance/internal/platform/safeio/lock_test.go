//go:build darwin || linux

package safeio_test

import (
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestLockExcludesAnotherOwner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "install.lock")
	first, err := safeio.Lock(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { first.Close() })
	if second, err := safeio.Lock(path); err == nil {
		second.Close()
		t.Fatal("concurrent lock owner accepted")
	}
	first.Close()
	second, err := safeio.Lock(path)
	if err != nil {
		t.Fatalf("released lock remained held: %v", err)
	}
	second.Close()
}

func TestLockRejectsSymlinksAndDevices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "linked.lock")
	if err := os.Symlink("/dev/null", path); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{path, "/dev/null", t.TempDir()} {
		if file, err := safeio.Lock(path); err == nil {
			file.Close()
			t.Errorf("accepted unsafe lock %s", path)
		}
	}
}

func TestPrivateDirDoesNotChmodSymlinkTarget(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bundle")
	if err := os.Symlink(directory, path); err != nil {
		t.Fatal(err)
	}
	if err := safeio.PrivateDir(path); err == nil {
		t.Fatal("followed directory symlink")
	}
	info, err := os.Stat(directory)
	if err != nil || info.Mode().Perm() != 0755 {
		t.Fatalf("changed unrelated target: %v, %v", info, err)
	}
}
