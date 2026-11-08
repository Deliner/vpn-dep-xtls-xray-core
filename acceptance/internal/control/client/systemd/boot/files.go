package boot

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

// Only fixed immediate children of the already protected systemd root are used.
// Existing directories are inspected; empty drop-in directories are retained.
func (t transaction) parent(item artifact, create bool) error {
	path := filepath.Dir(item.path)
	if path == t.units {
		return nil
	}
	if create {
		if err := os.Mkdir(path, 0755); err == nil {
			if err := os.Chmod(path, 0755); err != nil {
				return err
			}
			if err := safeio.SyncDir(t.units); err != nil {
				return err
			}
		} else if !os.IsExist(err) {
			return err
		}
	}
	err := safeio.CheckAccess(path, os.ModeDir|0755, t.uid, t.gid)
	if !create && os.IsNotExist(err) {
		return nil
	}
	return err
}

func (t transaction) verify(item artifact, required bool) (bool, error) {
	if err := t.parent(item, false); err != nil {
		return false, err
	}
	info, err := os.Lstat(item.path)
	if os.IsNotExist(err) && !required {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	mode := os.FileMode(0644)
	if item.link != "" {
		// Symlink permission bits are platform-specific; access is controlled by
		// the protected parent. Type, owner and literal target remain mandatory.
		mode = os.ModeSymlink | info.Mode().Perm()
	}
	if err := safeio.CheckAccess(item.path, mode, t.uid, t.gid); err != nil {
		return false, err
	}
	if item.link != "" {
		actual, err := os.Readlink(item.path)
		if err != nil || actual != item.link {
			return false, fmt.Errorf("foreign boot activation link: %s", item.path)
		}
		return true, nil
	}
	actual, err := safeio.Read(item.path, 65536)
	if err != nil || !bytes.Equal(actual, item.data) {
		return false, fmt.Errorf("foreign or changed boot artifact: %s", item.path)
	}
	return true, nil
}

func (t transaction) inspect(files []artifact, mode string) error {
	for _, item := range files {
		found, err := t.verify(item, mode == "enabled")
		if err != nil {
			return err
		}
		if found && mode == "absent" {
			return fmt.Errorf("unrecorded boot artifact exists; inspect before retrying: %s", item.path)
		}
	}
	return nil
}

func (t transaction) create(item artifact) error {
	found, err := t.verify(item, false)
	if err != nil || found {
		return err
	}
	if err := t.parent(item, true); err != nil {
		return err
	}
	if item.link == "" {
		return safeio.Create(item.path, item.data, 0644)
	}
	if err := os.Symlink(item.link, item.path); err != nil {
		return err
	}
	return safeio.SyncDir(filepath.Dir(item.path))
}

func (t transaction) remove(item artifact) error {
	found, err := t.verify(item, false)
	if err != nil || !found {
		return err
	}
	if err := os.Remove(item.path); err != nil {
		return err
	}
	return safeio.SyncDir(filepath.Dir(item.path))
}
