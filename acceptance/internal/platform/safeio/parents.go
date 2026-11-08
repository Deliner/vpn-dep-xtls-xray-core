package safeio

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// ProtectedParent rejects path replacement through foreign, writable or
// symlinked parents. Root-owned sticky directories retain normal /tmp semantics.
func ProtectedParent(path string, owner int) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || !info.IsDir() || int(stat.Uid) != owner && stat.Uid != 0 {
			return fmt.Errorf("parent must be a real directory owned by the expected user or root: %s", parent)
		}
		if info.Mode().Perm()&0022 != 0 && !(stat.Uid == 0 && info.Mode()&os.ModeSticky != 0) {
			return fmt.Errorf("parent permits untrusted replacement: %s", parent)
		}
		if parent == filepath.Dir(parent) {
			return nil
		}
	}
}
