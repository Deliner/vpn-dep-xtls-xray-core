package host

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"vpn-script/internal/platform/safeio"
)

func protectedParents(path string, owner int) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("client state requires a clean absolute parent path")
	}
	if path != "/" {
		if err := protectedParents(filepath.Dir(path), owner); err != nil {
			return err
		}
	}
	return protected(path, owner, true)
}

func protectedFile(path string, owner int) error { return protected(path, owner, false) }

// ProtectedDirectory verifies the entire root-owned path without changing it.
func ProtectedDirectory(path string) error { return protectedParents(path, 0) }

// ProtectedExecutable permits resolving a system-provided tool symlink only
// before checking the resulting root-owned executable and every parent.
func ProtectedExecutable(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if err := protectedParents(filepath.Dir(resolved), 0); err != nil {
		return "", err
	}
	return resolved, protectedFile(resolved, 0)
}

// PrivateInput prevents an unattended service from trusting writable parents or
// non-private credentials. Existing permissions are inspected, never repaired.
func PrivateInput(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("service input requires a clean absolute path")
	}
	if err := protectedParents(filepath.Dir(path), 0); err != nil {
		return err
	}
	return safeio.CheckAccess(path, 0600, 0, 0)
}

func protected(path string, owner int, directory bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != owner || info.Mode().Perm()&0022 != 0 ||
		info.Mode()&os.ModeSymlink != 0 || info.IsDir() != directory || !directory && !info.Mode().IsRegular() {
		return fmt.Errorf("unprotected client path: %s; inspect ownership, type and write access", path)
	}
	return nil
}

func directory(path string, mode os.FileMode) error {
	if err := os.Mkdir(path, mode); err == nil {
		if err := os.Chown(path, 0, 0); err != nil {
			return err
		}
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
	} else if !os.IsExist(err) {
		return err
	}
	return safeio.CheckAccess(path, os.ModeDir|mode, 0, 0)
}
