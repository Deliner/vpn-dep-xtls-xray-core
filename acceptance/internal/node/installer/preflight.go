package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"vpn-script/internal/platform/safeio"
)

func (i *Installer) preflight() error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return fmt.Errorf("installation requires Linux with systemd and root privileges")
	}
	if info, err := os.Stat("/run/systemd/system"); err != nil || !info.IsDir() {
		return fmt.Errorf("installation requires systemd (target: Ubuntu 24.04)")
	}
	for _, name := range []string{"systemctl", "ss", "useradd", "id", "getent"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("missing host prerequisite: %s", name)
		}
	}
	if err := i.managedPaths(); err != nil {
		return err
	}
	unit, err := optionalFile(i.Paths.Unit)
	if err != nil {
		return err
	}
	if unit != nil && !strings.HasPrefix(string(unit), marker) {
		return fmt.Errorf("refusing to replace an unmanaged unit")
	}
	return nil
}

func (i *Installer) managedPaths() error {
	if err := managedBase(i.Paths.Base); err != nil {
		return err
	}
	paths := map[string]os.FileMode{i.Paths.Unit: 0644, i.Paths.Firewall: 0644,
		filepath.Join(i.Paths.Base, "state.json"): 0600, filepath.Join(i.Paths.Base, "install.lock"): 0600,
		filepath.Join(i.Paths.Base, "last-error.log"): 0600, filepath.Join(i.Paths.Base, "cache"): os.ModeDir | 0700,
		filepath.Join(i.Paths.Base, "releases"): os.ModeDir | 0750}
	for path, mode := range paths {
		if err := safeio.CheckAccess(path, mode, os.Geteuid(), -1); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func managedBase(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0700); err != nil {
			return err
		}
		return safeio.CheckAccess(path, os.ModeDir|0700, os.Geteuid(), -1)
	}
	if err != nil {
		return err
	}
	if info.Mode() != os.ModeDir|0700 && info.Mode() != os.ModeDir|0750 {
		return fmt.Errorf("unsafe managed access: %s; expected private directory; inspect before retrying", path)
	}
	return safeio.CheckAccess(path, info.Mode(), os.Geteuid(), -1)
}

func Architecture() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil
	default:
		return "", fmt.Errorf("only amd64 and arm64 are supported")
	}
}
