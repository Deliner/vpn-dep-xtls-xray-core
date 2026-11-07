//go:build darwin || linux

package safeio

import (
	"fmt"
	"os"
	"syscall"
)

// CheckAccess verifies existing metadata without repairing or following a link.
// group=-1 omits that check; callers must separately verify any privileged group.
// Callers must first protect parent directories against untrusted replacement.
func CheckAccess(path string, mode os.FileMode, owner, group int) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot inspect managed ownership: %s", path)
	}
	if info.Mode() != mode || int(stat.Uid) != owner || group >= 0 && int(stat.Gid) != group {
		expected := fmt.Sprintf("uid %d", owner)
		if group >= 0 {
			expected += fmt.Sprintf(", gid %d", group)
		}
		return fmt.Errorf("unsafe managed access: %s (mode %s, uid %d, gid %d; expected mode %s, %s); inspect before retrying",
			path, info.Mode(), stat.Uid, stat.Gid, mode, expected)
	}
	return nil
}
