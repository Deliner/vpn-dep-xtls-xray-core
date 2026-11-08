package boot

import (
	"context"
	"os"
	"path/filepath"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/systemd/install"
	"vpn-script/internal/control/client/systemd/manager"
)

func Enable(ctx context.Context, networkManager string) (Receipt, error) {
	t, err := openTransaction()
	if err != nil {
		return Receipt{}, err
	}
	defer t.snapshot.Close()
	if err := t.snapshot.VerifyExecutable(); err != nil {
		return Receipt{}, err
	}
	if err := host.VerifyTools("nft"); err != nil {
		return Receipt{}, err
	}
	return t.enable(ctx, networkManager)
}

func Disable(ctx context.Context) (Receipt, error) {
	t, err := openTransaction()
	if err != nil {
		return Receipt{}, err
	}
	defer t.snapshot.Close()
	return t.disable(ctx)
}

func openTransaction() (transaction, error) {
	snapshot, err := install.LockSnapshot()
	if err != nil {
		return transaction{}, err
	}
	command, err := manager.New()
	if err != nil {
		snapshot.Close()
		return transaction{}, err
	}
	return transaction{snapshot: snapshot, units: filepath.Dir(install.UnitPath),
		state: install.BootStatePath, uid: os.Geteuid(), gid: os.Getegid(), command: command}, nil
}
