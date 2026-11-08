package install

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/safeio"
)

func openStore(base, unit string, create bool) (*store, error) {
	s := &store{base: base, unit: unit, owner: os.Geteuid()}
	if err := safeio.ProtectedParent(base, s.owner); err != nil {
		return nil, err
	}
	if err := safeio.ProtectedParent(unit, s.owner); err != nil {
		return nil, err
	}
	if create {
		if err := s.initialize(); err != nil {
			return nil, err
		}
	}
	if err := s.access(base, os.ModeDir|0700); err != nil {
		return nil, err
	}
	var owner ownership
	if err := s.read(filepath.Join(base, "owner.json"), &owner); err != nil || owner.Version != 1 || !credential.ValidToken(owner.Token) {
		return nil, fmt.Errorf("Fleet service ownership record is missing or invalid")
	}
	s.token = owner.Token
	if err := s.access(filepath.Join(base, "releases"), os.ModeDir|0700); err != nil {
		return nil, err
	}
	var err error
	s.lock, err = safeio.Lock(filepath.Join(base, "install.lock"))
	return s, err
}

func (s *store) initialize() error {
	if _, err := os.Lstat(s.base); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if _, err := os.Lstat(s.unit); !os.IsNotExist(err) {
		return fmt.Errorf("unowned Fleet service unit already exists or cannot be inspected")
	}
	if err := os.Mkdir(s.base, 0700); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(s.base, "releases"), 0700); err != nil {
		return err
	}
	token, err := credential.NewToken()
	if err != nil {
		return err
	}
	if err := safeio.NewJSON(filepath.Join(s.base, "owner.json"), ownership{Version: 1, Token: token}); err != nil {
		return err
	}
	return safeio.SyncDir(filepath.Dir(s.base))
}

func (s *store) access(path string, mode os.FileMode) error {
	return safeio.CheckAccess(path, mode, s.owner, -1)
}

func (s *store) read(path string, value any) error {
	if err := s.access(path, 0600); err != nil {
		return err
	}
	return safeio.LoadJSON(path, value)
}
