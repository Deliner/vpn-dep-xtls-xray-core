package app

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"vpn-script/internal/node/supervisor"
)

// The existing v2 installer copies its executable into the immutable release
// as nhop-node. Keep that executable's systemd --supervise contract intact.
func supervise(ctx context.Context, args []string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("installed Fleet supervisor requires Linux")
	}
	if len(args) != 2 || args[1] == "" {
		return fmt.Errorf("use --supervise RELEASE_DIRECTORY")
	}
	err := supervisor.Run(ctx, args[1])
	if errors.Is(err, context.Canceled) && ctx.Err() == context.Canceled {
		return nil
	}
	return err
}
