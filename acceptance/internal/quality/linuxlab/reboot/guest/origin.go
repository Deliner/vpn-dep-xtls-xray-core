package guest

import (
	"context"
	"fmt"
	"os"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func originAddress(ctx context.Context, config contract.Config, remove bool) error {
	path := contract.Root + "/origin-owner"
	if !remove {
		if _, err := Command(ctx, "/usr/sbin/ip", "address", "add", contract.OriginIP+"/32", "dev", "lo"); err != nil {
			return err
		}
		return safeio.Create(path, []byte(config.Token), 0600)
	}
	if err := safeio.CheckAccess(path, 0600, 0, 0); err != nil {
		return err
	}
	owner, err := os.ReadFile(path)
	if err != nil || string(owner) != config.Token {
		return fmt.Errorf("origin address ownership differs")
	}
	if _, err := Command(ctx, "/usr/sbin/ip", "address", "del", contract.OriginIP+"/32", "dev", "lo"); err != nil {
		return err
	}
	return os.Remove(path)
}
