package install

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/client/systemd"
	"vpn-script/internal/platform/safeio"
)

func (s *store) stage(b bundle) (Receipt, []byte, error) {
	id, err := safeio.Digest(b.identity)
	if err != nil {
		return Receipt{}, nil, err
	}
	r := Receipt{Generation: id, Unit: s.unit, Directory: filepath.Join(Directory, "releases", id)}
	destination := filepath.Join(s.base, "releases", id)
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		unit, err := s.generation(id)
		return r, unit, err
	}
	unit, err := s.unitFor(r.Directory, id, b.identity)
	if err != nil {
		return r, nil, err
	}
	staging, err := os.MkdirTemp(filepath.Join(s.base, "releases"), ".stage-")
	if err != nil {
		return r, nil, err
	}
	// Only this newly created, never executed directory is eligible for recursive
	// cleanup. Published generations are retained, even after failed reloads.
	defer os.RemoveAll(staging)
	if err := writeGeneration(staging, r.Directory, b, unit); err != nil {
		return r, nil, err
	}
	if err := os.Rename(staging, destination); err != nil {
		return r, nil, err
	}
	if err := safeio.SyncDir(filepath.Dir(destination)); err != nil {
		return r, nil, err
	}
	verified, err := s.generation(id)
	return r, verified, err
}

func (s *store) unitFor(directory, id string, value identity) ([]byte, error) {
	unit, err := systemd.Unit(systemd.Options{Executable: filepath.Join(directory, "nhop-client"), Profile: filepath.Join(directory, "profile.json"),
		Runtimes: filepath.Join(directory, "runtimes.json"), Network: value.Network})
	if err != nil {
		return nil, err
	}
	return append([]byte(fmt.Sprintf("%s%s %s\n", marker, s.owner.Token, id)), unit...), nil
}

func writeGeneration(staging, destination string, b bundle, unit []byte) error {
	for name, data := range b.files {
		mode := os.FileMode(0755)
		if name == "profile.json" {
			mode = 0600
		}
		if err := safeio.Create(filepath.Join(staging, name), data, mode); err != nil {
			return err
		}
	}
	if err := safeio.NewJSON(filepath.Join(staging, "runtimes.json"), runtimeInventory(destination, b.identity)); err != nil {
		return err
	}
	if err := safeio.Create(filepath.Join(staging, "unit"), unit, 0600); err != nil {
		return err
	}
	return safeio.NewJSON(filepath.Join(staging, "release.json"), release{Identity: b.identity, UnitHash: safeio.Hash(unit)})
}
