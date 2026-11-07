package host

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func TestWritableOrLinkedStateIsRejectedWithoutRepair(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "state")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := protected(path, os.Geteuid(), true); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0777); err != nil {
		t.Fatal(err)
	}
	if protected(path, os.Geteuid(), true) == nil {
		t.Fatal("writable state was accepted")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0777 {
		t.Fatal("unsafe state was silently repaired")
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if protected(link, os.Geteuid(), true) == nil {
		t.Fatal("linked state was adopted")
	}
	if protectedParents(path+"/../state", os.Geteuid()) == nil {
		t.Fatal("noncanonical path accepted")
	}
}

func TestLookupFailureNeverCreatesServiceUser(t *testing.T) {
	calls := 0
	run := func(_ context.Context, _ time.Duration, args ...string) (process.Result, error) {
		calls++
		if args[0] != "getent" {
			t.Fatal("account mutation after a lookup command failure", args)
		}
		return process.Result{}, fmt.Errorf("unavailable command")
	}
	if _, err := identity(context.Background(), run); err == nil || calls != 1 {
		t.Fatal("failed identity lookup accepted", err, calls)
	}
}
