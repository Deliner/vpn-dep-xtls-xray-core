package systemd_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/platform/safeio"
)

func (f *bootFixture) close() {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if !f.removeUnits(ctx) {
		return
	}
	if f.child != nil && f.child.Process != nil {
		f.child.Process.Kill()
		f.child.Wait()
	}
	if f.mounted {
		if data, err := exec.CommandContext(ctx, "umount", host.Directory).CombinedOutput(); err != nil {
			f.t.Error("cannot release private boot fixture mount", err, string(data))
			return
		}
	}
	if err := os.RemoveAll(f.root); err != nil {
		f.t.Error("cannot remove newly created private boot fixture", err)
	}
}

func (f *bootFixture) removeUnits(ctx context.Context) bool {
	f.t.Helper()
	var owned []string
	for path, expected := range f.files {
		actual, err := safeio.Read(path, 65536)
		if err != nil || !bytes.Equal(actual, expected) {
			f.t.Error("refusing cleanup of changed test unit", path, err)
			return false
		}
		owned = append(owned, filepath.Base(path))
	}
	if f.command != nil && len(owned) > 0 {
		if _, err := f.command(ctx, append([]string{"stop", "--"}, owned...)...); err != nil {
			f.t.Error("cannot stop owned boot test units", err)
			return false
		}
	}
	for path := range f.files {
		if err := os.Remove(path); err != nil {
			f.t.Error(err)
			return false
		}
	}
	if f.command != nil {
		if _, err := f.command(ctx, "daemon-reload"); err != nil {
			f.t.Error(err)
			return false
		}
	}
	return true
}
