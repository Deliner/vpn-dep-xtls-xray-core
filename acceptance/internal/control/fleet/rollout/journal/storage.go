package journal

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

func Open(root string) (*Store, error) {
	if err := safeio.ProtectedParent(root, os.Geteuid()); err != nil {
		return nil, err
	}
	if err := safeio.CheckAccess(root, os.ModeDir|0700, os.Geteuid(), -1); err != nil {
		return nil, err
	}
	lock, err := safeio.Lock(filepath.Join(root, "rollout.lock"))
	if err != nil {
		return nil, err
	}
	s := &Store{root: root, lock: lock}
	if err := s.initialize(); err != nil {
		lock.Close()
		return nil, err
	}
	for _, name := range []string{"candidates", "plans"} {
		if err := safeio.CheckAccess(filepath.Join(root, name), os.ModeDir|0700, os.Geteuid(), -1); err != nil {
			lock.Close()
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.lock.Close()
}

func (s *Store) initialize() error {
	path := filepath.Join(s.root, "format.json")
	if _, err := os.Lstat(path); err == nil {
		var value struct {
			Version int `json:"version"`
		}
		if err := readPrivate(path, &value); err != nil || value.Version != 1 {
			return fmt.Errorf("invalid rollout journal format")
		}
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	entries, err := os.ReadDir(s.root)
	if err != nil || len(entries) != 1 || entries[0].Name() != "rollout.lock" {
		return fmt.Errorf("rollout journal format is missing; refusing to reset existing state")
	}
	for _, name := range []string{"candidates", "plans"} {
		if err := os.Mkdir(filepath.Join(s.root, name), 0700); err != nil {
			return err
		}
	}
	return safeio.NewJSON(path, struct {
		Version int `json:"version"`
	}{Version: 1})
}

func readPrivate(path string, value any) error {
	if err := safeio.CheckAccess(path, 0600, os.Geteuid(), -1); err != nil {
		return err
	}
	return safeio.LoadJSON(path, value)
}
