package guest

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func Command(parent context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 25*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, args...)
	command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL=C", "SYSTEMD_PAGER=cat", "SYSTEMD_COLORS=0"}
	data, err := command.CombinedOutput()
	if err != nil {
		return data, fmt.Errorf("%s failed: %w: %s", name, err, data)
	}
	return data, nil
}

func Client(ctx context.Context, args ...string) ([]byte, error) {
	return Command(ctx, contract.Root+"/bin/nhop-client", args...)
}

func Manager(ctx context.Context, args ...string) ([]byte, error) {
	return Command(ctx, "/usr/bin/systemctl", args...)
}
