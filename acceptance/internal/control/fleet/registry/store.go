package registry

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"time"

	"vpn-script/internal/platform/credential"
	"vpn-script/internal/platform/safeio"
)

func Open(directory string) (*Store, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{".", "nodes", "assignments", "rollouts"} {
		if err := safeio.CheckAccess(filepath.Join(root, name), os.ModeDir|0700, os.Geteuid(), -1); err != nil {
			return nil, err
		}
	}
	lock, err := safeio.Lock(filepath.Join(root, "registry.lock"))
	if err != nil {
		return nil, err
	}
	value, err := readAuthority(root)
	if err != nil {
		lock.Close()
		return nil, err
	}
	return &Store{directory: root, authority: value, key: ed25519.NewKeyFromSeed(value.Seed), lock: lock, now: time.Now}, nil
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lock == nil {
		return nil
	}
	err := s.lock.Close()
	s.lock = nil
	clear(s.key)
	clear(s.authority.Seed)
	return err
}

func (s *Store) AuthenticateAdmin(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lock != nil && credential.Matches(s.authority.AdminHash, token)
}

func (s *Store) available() error {
	if s.lock == nil {
		return ErrClosed
	}
	return nil
}
