package pull

import (
	"os"
	"path/filepath"

	"vpn-script/internal/node/fleet"
	"vpn-script/internal/platform/fleetapi"
	"vpn-script/internal/platform/safeio"
)

func Open(directory string) (*Runner, error) {
	directory, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	if err := safeio.CheckAccess(directory, os.ModeDir|0700, os.Geteuid(), -1); err != nil {
		return nil, err
	}
	lock, err := safeio.Lock(filepath.Join(directory, "pull.lock"))
	if err != nil {
		return nil, err
	}
	runner, err := openLocked(directory, lock)
	if err != nil {
		lock.Close()
	}
	return runner, err
}

func openLocked(directory string, lock *os.File) (*Runner, error) {
	value, err := readConnection(directory)
	if err != nil {
		return nil, err
	}
	agent := fleet.New(filepath.Join(directory, "agent"), value.Trust)
	if _, err := agent.Status(); err != nil {
		return nil, err
	}
	client, err := fleetapi.New(value.Endpoint, value.CAPEM)
	if err != nil {
		return nil, err
	}
	return &Runner{directory: directory, config: value, lock: lock, client: client, agent: agent}, nil
}

func (r *Runner) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lock == nil {
		return nil
	}
	r.client.Close()
	err := r.lock.Close()
	r.lock = nil
	return err
}
