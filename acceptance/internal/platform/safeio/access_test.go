package safeio

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestCheckAccessRejectsMetadataDriftWithoutRepair(t *testing.T) {
	for _, mode := range []os.FileMode{0644, 0660, os.ModeSetuid | 0600, os.ModeSticky | 0600} {
		t.Run(mode.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "secret")
			if err := Atomic(path, []byte("secret"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			err = CheckAccess(path, 0600, os.Geteuid(), os.Getegid())
			if err == nil || !strings.Contains(err.Error(), "unsafe managed access") {
				t.Fatal("unsafe permissions accepted", err)
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
				t.Fatal("permission inspection modified the file", err)
			}
		})
	}
}

func TestCheckAccessChecksOwnerAndPrivilegedGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := Atomic(path, []byte("config"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := CheckAccess(path, 0640, os.Geteuid(), os.Getegid()); err != nil {
		t.Fatal(err)
	}
	if err := CheckAccess(path, 0640, os.Geteuid()+1, -1); err == nil {
		t.Fatal("foreign owner accepted")
	}
	if err := CheckAccess(path, 0640, os.Geteuid(), os.Getegid()+1); err == nil {
		t.Fatal("foreign privileged group accepted")
	}
	if err := CheckAccess(path, 0640, os.Geteuid(), -1); err != nil {
		t.Fatal("explicitly omitted group check was applied", err)
	}
	if err := CheckAccess(path+"-missing", 0600, os.Geteuid(), -1); !os.IsNotExist(err) {
		t.Fatal("missing file error was not preserved", err)
	}
}

func TestCheckAccessNeverFollowsSymlinksOrReadsSpecialFiles(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"symlink", "fifo", "directory"} {
		path := filepath.Join(directory, name)
		var err error
		switch name {
		case "symlink":
			err = os.Symlink(filepath.Join(directory, "must-not-create"), path)
		case "fifo":
			err = syscall.Mkfifo(path, 0600)
		case "directory":
			err = os.Mkdir(path, 0700)
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := CheckAccess(path, 0600, os.Geteuid(), -1); err == nil {
			t.Fatalf("non-regular file %s accepted", name)
		}
	}
	if _, err := os.Lstat(filepath.Join(directory, "must-not-create")); !os.IsNotExist(err) {
		t.Fatal("symbolic link target changed", err)
	}
	if err := CheckAccess(directory, os.ModeDir|0700, os.Geteuid(), -1); err != nil {
		t.Fatal(err)
	}
}
