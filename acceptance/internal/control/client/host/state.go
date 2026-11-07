package host

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

const Directory = "/run/nhop-client"
const Socket = Directory + "/control/command.sock"

type Owner struct {
	lock     *os.File
	previous []byte
}

// Acquire never chmods or adopts an existing foreign directory or lock file.
// The lock covers account creation, session ownership and the control socket.
func Acquire() (*Owner, error) {
	if err := LinuxRoot(); err != nil {
		return nil, err
	}
	if err := protectedParents(filepath.Dir(Directory), 0); err != nil {
		return nil, err
	}
	if err := directory(Directory, 0711); err != nil {
		return nil, err
	}
	path := filepath.Join(Directory, "controller.lock")
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		if err := safeio.CheckAccess(path, 0600, 0, 0); err != nil {
			return nil, err
		}
	}
	lock, err := safeio.Lock(path)
	if err != nil {
		return nil, fmt.Errorf("client controller already active or lock inaccessible: %w", err)
	}
	if err := safeio.CheckAccess(path, 0600, 0, 0); err != nil {
		lock.Close()
		return nil, err
	}
	return &Owner{lock: lock}, nil
}

func (o *Owner) Close() error { return o.lock.Close() }

func (o *Owner) PrepareSocket() error {
	if err := directory(filepath.Dir(Socket), 0700); err != nil {
		return err
	}
	if _, err := os.Lstat(Socket); os.IsNotExist(err) {
		return nil
	}
	if err := safeio.CheckAccess(Socket, os.ModeSocket|0600, 0, 0); err != nil {
		return err
	}
	return os.Remove(Socket)
}

func InspectSocket() error {
	if err := LinuxRoot(); err != nil {
		return err
	}
	if err := protectedParents(filepath.Dir(Socket), 0); err != nil {
		return err
	}
	return safeio.CheckAccess(Socket, os.ModeSocket|0600, 0, 0)
}
