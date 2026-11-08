// Package manager binds privileged operations to the local system manager.
package manager

import (
	"context"
	"os/exec"
	"time"
	"vpn-script/internal/control/client/host"
)

type Command func(context.Context, ...string) ([]byte, error)

func New() (Command, error) {
	path, err := host.ProtectedExecutable("/usr/bin/systemctl")
	if err != nil {
		return nil, err
	}
	return func(parent context.Context, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(parent, 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, path, args...)
		// Ignore remote/user bus, pager, unit-path and loader overrides. All calls
		// address the local system manager and fixed unit name only.
		command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL=C", "SYSTEMD_PAGER=cat", "SYSTEMD_COLORS=0"}
		return command.Output()
	}, nil
}
