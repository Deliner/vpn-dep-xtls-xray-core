package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/session"
)

func Apply(ctx context.Context, options Options) (Receipt, error) {
	owner, err := idleController()
	if err != nil {
		return Receipt{}, err
	}
	defer owner.Close()
	for _, path := range []string{options.Profile, options.Runtimes} {
		if err := host.PrivateInput(path); err != nil {
			return Receipt{}, err
		}
	}
	executable, err := runningExecutable()
	if err != nil {
		return Receipt{}, err
	}
	b, err := loadBundle(options, executable)
	if err != nil {
		return Receipt{}, err
	}
	t, err := openTransaction()
	if err != nil {
		return Receipt{}, err
	}
	defer t.store.lock.Close()
	return t.apply(ctx, b)
}

// Remove unregisters only the verified unit. It retains all snapshots and the
// controller journal; it never implicitly disconnects or deletes recovery data.
func Remove(ctx context.Context) error {
	owner, err := idleController()
	if err != nil {
		return err
	}
	defer owner.Close()
	if _, err := os.Lstat(Directory); err != nil {
		return fmt.Errorf("no accessible owned client installation: %w", err)
	}
	t, err := openTransaction()
	if err != nil {
		return err
	}
	defer t.store.lock.Close()
	return t.remove(ctx)
}

func idleController() (*host.Owner, error) {
	if err := host.LinuxRoot(); err != nil {
		return nil, err
	}
	if os.Getegid() != 0 {
		return nil, fmt.Errorf("client installation requires the root primary group")
	}
	owner, err := host.Acquire()
	if err != nil {
		return nil, err
	}
	if err := stoppedJournal(owner); err != nil {
		owner.Close()
		return nil, err
	}
	return owner, nil
}

func stoppedJournal(owner *host.Owner) error {
	directory, err := owner.Current()
	if err != nil || directory == "" {
		return err
	}
	var status session.Status
	if err := host.ReadSession(directory, "status.json", &status); err != nil {
		return err
	}
	if status.Phase != "stopped" {
		return fmt.Errorf("client session retains protection; explicit stop required before changing service installation")
	}
	return nil
}

func openTransaction() (transaction, error) {
	for _, path := range []string{filepath.Dir(Directory), filepath.Dir(UnitPath)} {
		if err := host.ProtectedDirectory(path); err != nil {
			return transaction{}, err
		}
	}
	manager, err := systemManager()
	if err != nil {
		return transaction{}, err
	}
	s, err := openStore(Directory, UnitPath)
	return transaction{store: s, manager: manager}, err
}
