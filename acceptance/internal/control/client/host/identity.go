// Package host owns the Linux client's local account and protected control state.
package host

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"vpn-script/internal/platform/accounts"
	"vpn-script/internal/platform/process"
)

const Account = "nhop-client"

func LinuxRoot() error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return fmt.Errorf("client control requires Linux root; no host settings changed")
	}
	return nil
}

// Identity creates only a missing named account. Existing NSS records are never
// repaired or borrowed; both numeric IDs must belong to this non-login identity.
func Identity(ctx context.Context) (accounts.Identity, error) {
	if err := LinuxRoot(); err != nil {
		return accounts.Identity{}, err
	}
	return identity(ctx, process.Run)
}

func identity(ctx context.Context, run process.Runner) (accounts.Identity, error) {
	result, err := run(ctx, 5*time.Second, "getent", "passwd", Account)
	if err != nil || !result.Success {
		// Require getent's key-not-found exit status. Other command failures
		// must not be interpreted as permission to change users.
		if !missing(err) {
			return accounts.Identity{}, fmt.Errorf("cannot determine whether client account exists")
		}
		result, err = run(ctx, 10*time.Second, "useradd", "--system", "--user-group", "--no-create-home",
			"--home-dir", accounts.Home, "--shell", accounts.Shell, Account)
		if err != nil || !result.Success {
			return accounts.Identity{}, fmt.Errorf("cannot create dedicated client account; inspect NSS before retrying")
		}
	}
	command := func(args ...string) ([]byte, error) {
		result, err := run(ctx, 5*time.Second, args...)
		if err != nil || !result.Success {
			return nil, fmt.Errorf("client identity lookup failed")
		}
		return result.Output, nil
	}
	return (accounts.Policy{Name: Account, MinimumID: 100}).Inspect(command)
}
