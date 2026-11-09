package install

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"vpn-script/internal/core/validate"
	"vpn-script/internal/node/fleet/service"
	"vpn-script/internal/platform/safeio"
)

func (s *store) publish(options Options, binary []byte) (string, error) {
	value := identity{Version: 1, State: options.State, IntervalSeconds: int64(options.Interval / time.Second), BinarySHA256: safeio.Hash(binary)}
	id, err := safeio.Digest(value)
	if err != nil {
		return "", err
	}
	directory := filepath.Join(s.base, "releases", id)
	// The unit names the permanent installation; the store owns staged file access.
	executable := filepath.Join(service.Directory, "releases", id, "nhop-fleet")
	unit, err := service.Unit(executable, value.State, options.Interval)
	if err != nil {
		return "", err
	}
	unit = append([]byte(marker+s.token+" "+id+"\n"), unit...)
	if _, err := os.Lstat(directory); !os.IsNotExist(err) {
		_, current, err := s.revision(id)
		if err != nil || !bytes.Equal(current, unit) {
			return "", fmt.Errorf("existing Fleet service generation differs or is unreadable")
		}
		return id, nil
	}
	stage, err := os.MkdirTemp(filepath.Join(s.base, "releases"), ".prepare-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	if err := writeRevision(stage, binary, unit, revision{Identity: value, UnitSHA256: safeio.Hash(unit)}); err != nil {
		return "", err
	}
	if err := os.Rename(stage, directory); err != nil {
		return "", err
	}
	return id, safeio.SyncDir(filepath.Dir(directory))
}

func writeRevision(directory string, binary, unit []byte, value revision) error {
	if err := safeio.Create(filepath.Join(directory, "nhop-fleet"), binary, 0700); err != nil {
		return err
	}
	if err := safeio.Create(filepath.Join(directory, "unit.txt"), unit, 0600); err != nil {
		return err
	}
	return safeio.NewJSON(filepath.Join(directory, "revision.json"), value)
}

func (s *store) revision(id string) (revision, []byte, error) {
	if !validate.Hash(id) {
		return revision{}, nil, fmt.Errorf("invalid Fleet service generation")
	}
	directory := filepath.Join(s.base, "releases", id)
	if err := s.access(directory, os.ModeDir|0700); err != nil {
		return revision{}, nil, err
	}
	var value revision
	if err := s.read(filepath.Join(directory, "revision.json"), &value); err != nil {
		return revision{}, nil, err
	}
	observed, err := safeio.Digest(value.Identity)
	if err != nil || observed != id || value.Identity.Version != 1 || service.PermanentPath(value.Identity.State) != nil || value.Identity.IntervalSeconds < 1 || value.Identity.IntervalSeconds > 3600 {
		return revision{}, nil, fmt.Errorf("Fleet service generation identity mismatch")
	}
	if _, err := s.verifiedFile(filepath.Join(directory, "nhop-fleet"), 0700, 100_000_000, value.Identity.BinarySHA256); err != nil {
		return revision{}, nil, err
	}
	unit, err := s.verifiedFile(filepath.Join(directory, "unit.txt"), 0600, 65536, value.UnitSHA256)
	if err == nil && !bytes.HasPrefix(unit, []byte(marker+s.token+" "+id+"\n")) {
		err = fmt.Errorf("Fleet service generation belongs to another installation")
	}
	return value, unit, err
}

func (s *store) verifiedFile(path string, mode os.FileMode, limit int64, hash string) ([]byte, error) {
	if err := s.access(path, mode); err != nil {
		return nil, err
	}
	raw, err := safeio.Read(path, limit)
	if err != nil || !validate.Hash(hash) || safeio.Hash(raw) != hash {
		return nil, fmt.Errorf("Fleet service file checksum differs: %s", path)
	}
	return raw, nil
}
