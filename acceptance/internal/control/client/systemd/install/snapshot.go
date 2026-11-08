package install

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/platform/safeio"
)

// Snapshot keeps both controller and installation locks until Close. Boot
// registration consumes verified immutable inputs without owning their format.
type Snapshot struct {
	Receipt
	OwnerToken string
	controller *host.Owner
	lock       *os.File
}

func LockSnapshot() (*Snapshot, error) {
	owner, err := idleController()
	if err != nil {
		return nil, err
	}
	value, err := installedSnapshot()
	if err != nil {
		owner.Close()
		return nil, err
	}
	value.controller = owner
	return value, nil
}

func installedSnapshot() (*Snapshot, error) {
	if _, err := os.Lstat(Directory); err != nil {
		return nil, fmt.Errorf("no accessible owned client installation: %w", err)
	}
	t, err := openTransaction()
	if err != nil {
		return nil, err
	}
	value, err := t.store.snapshot()
	if err != nil {
		t.store.lock.Close()
		return nil, err
	}
	value.lock = t.store.lock
	return value, nil
}

func (s *store) snapshot() (*Snapshot, error) {
	unit, err := s.current()
	if err != nil {
		return nil, err
	}
	if len(unit) == 0 {
		return nil, fmt.Errorf("client service is not installed")
	}
	line, _, _ := strings.Cut(string(unit), "\n")
	id := strings.TrimPrefix(line, marker+s.owner.Token+" ")
	return &Snapshot{Receipt: Receipt{Generation: id, Unit: s.unit,
		Directory: filepath.Join(Directory, "releases", id)}, OwnerToken: s.owner.Token}, nil
}

func (s *Snapshot) Close() error {
	return errors.Join(s.lock.Close(), s.controller.Close())
}

// Enabling boot must use the installed CLI version, whose boot command contract
// was verified with this installation. Recovery/disable does not require it.
func (s *Snapshot) VerifyExecutable() error {
	data, err := runningExecutable()
	if err != nil {
		return err
	}
	expected, err := safeio.FileHash(filepath.Join(s.Directory, "nhop-client"))
	if err != nil {
		return err
	}
	if safeio.Hash(data) != expected {
		return fmt.Errorf("enable boot with the installed client executable or reinstall this version first")
	}
	return nil
}
