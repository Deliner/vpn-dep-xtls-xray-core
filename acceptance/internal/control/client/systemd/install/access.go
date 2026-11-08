package install

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"vpn-script/internal/platform/safeio"
)

// The caller protects both parent directories before opening this store. Tests
// supply their own private parents; the public API always uses root-owned paths.
func openStore(base, unit string) (*store, error) {
	s := &store{base: base, unit: unit, uid: os.Geteuid(), gid: os.Getegid()}
	if err := os.Mkdir(base, 0700); err == nil {
		if err := s.initialize(); err != nil {
			return nil, err
		}
	} else if !os.IsExist(err) {
		return nil, err
	}
	if err := s.access(base, os.ModeDir|0700); err != nil {
		return nil, err
	}
	if err := s.readOwner(); err != nil {
		return nil, err
	}
	path := filepath.Join(base, "install.lock")
	if err := s.access(path, 0600); err != nil {
		return nil, err
	}
	lock, err := safeio.Lock(path)
	if err != nil {
		return nil, fmt.Errorf("client installation is locked: %w", err)
	}
	s.lock = lock
	return s, nil
}

func (s *store) access(path string, mode os.FileMode) error {
	return safeio.CheckAccess(path, mode, s.uid, s.gid)
}

func (s *store) initialize() error {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	value := ownership{SchemaVersion: 1, Token: hex.EncodeToString(token[:])}
	if err := os.Mkdir(filepath.Join(s.base, "releases"), 0700); err != nil {
		return err
	}
	if err := safeio.Create(filepath.Join(s.base, "install.lock"), nil, 0600); err != nil {
		return err
	}
	if err := safeio.NewJSON(filepath.Join(s.base, "owner.json"), value); err != nil {
		return err
	}
	return safeio.SyncDir(filepath.Dir(s.base))
}

func (s *store) readOwner() error {
	path := filepath.Join(s.base, "owner.json")
	if err := s.access(path, 0600); err != nil {
		return fmt.Errorf("unowned client installation; inspect before retrying: %w", err)
	}
	if err := safeio.LoadJSON(path, &s.owner); err != nil {
		return err
	}
	if s.owner.SchemaVersion != 1 || !regexp.MustCompile(`^[a-f0-9]{32}$`).MatchString(s.owner.Token) {
		return fmt.Errorf("invalid client installation ownership")
	}
	return s.access(filepath.Join(s.base, "releases"), os.ModeDir|0700)
}
