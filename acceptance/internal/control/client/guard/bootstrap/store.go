package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/client/guard"
	"vpn-script/internal/control/client/host"
	"vpn-script/internal/platform/safeio"
)

func access(create bool) (*store, error) {
	if err := host.LinuxRoot(); err != nil {
		return nil, err
	}
	if os.Getegid() != 0 {
		return nil, fmt.Errorf("bootstrap protection requires the root primary group")
	}
	if err := host.VerifyTools("nft"); err != nil {
		return nil, err
	}
	if err := host.ProtectedDirectory(host.Directory); err != nil && (create || !os.IsNotExist(err)) {
		return nil, err
	}
	m, err := guard.NewBootstrap()
	if err != nil {
		return nil, err
	}
	return openStore(Directory, create, m)
}

func openStore(base string, create bool, manager firewall) (*store, error) {
	s := &store{base: base, uid: os.Geteuid(), gid: os.Getegid(), guard: manager}
	if create {
		if err := os.Mkdir(base, 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
	}
	if err := safeio.CheckAccess(base, os.ModeDir|0700, s.uid, s.gid); err != nil {
		if !create && os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if err := s.members(); err != nil {
		return nil, err
	}
	path := filepath.Join(base, "lock")
	if _, err := os.Lstat(path); err == nil {
		if err := safeio.CheckAccess(path, 0600, s.uid, s.gid); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	lock, err := safeio.Lock(path)
	if err != nil {
		return nil, fmt.Errorf("bootstrap state is locked or inaccessible: %w", err)
	}
	if err := safeio.CheckAccess(path, 0600, s.uid, s.gid); err != nil {
		lock.Close()
		return nil, err
	}
	s.lock = lock
	return s, nil
}

func (s *store) members() error {
	entries, err := os.ReadDir(s.base)
	if err != nil {
		return err
	}
	for _, item := range entries {
		if item.Name() != "lock" && item.Name() != "state.json" {
			return fmt.Errorf("unexpected bootstrap state member; inspect before retrying")
		}
		if err := safeio.CheckAccess(filepath.Join(s.base, item.Name()), 0600, s.uid, s.gid); err != nil {
			return err
		}
	}
	return nil
}
