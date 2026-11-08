package guest

import (
	"context"
	"fmt"
	"os"
	"strings"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func Units(config contract.Config) map[string]string {
	marker := "# Owned reboot fixture " + config.Token + "\n"
	start := "ExecStart=/usr/bin/env NHOP_REBOOT_GUEST=owned-vm " + contract.Root + "/reboot.test -test.v -test.run=^"
	common := "DefaultDependencies=no\nAfter=local-fs.target\nBefore=shutdown.target\nConflicts=shutdown.target\n"
	return map[string]string{
		contract.PeerUnit:      marker + "[Unit]\nDescription=Owned reboot exit fixture\nAfter=network-online.target\nWants=network-online.target\n[Service]\nType=exec\n" + start + "TestRebootPeer$ -test.timeout=30m\nKillMode=control-group\nTimeoutStopSec=15s\nUMask=0077\n",
		contract.CanaryUnit:    marker + "[Unit]\n" + common + "Before=nhop-client-boot.service\n[Service]\nType=notify\nNotifyAccess=main\n" + start + "TestRebootCanary$ -test.timeout=5m\nTimeoutStartSec=15s\nTimeoutStopSec=10s\nUMask=0077\n[Install]\nWantedBy=multi-user.target\n",
		contract.RecoveryUnit:  marker + "[Unit]\n" + common + "[Service]\nType=oneshot\n" + start + "TestRebootRecovery$ -test.timeout=3m\nTimeoutStartSec=180s\nUMask=0077\n",
		contract.RecoveryTimer: marker + "[Unit]\n" + common + "[Timer]\nOnBootSec=90s\nAccuracySec=1s\nUnit=" + contract.RecoveryUnit + "\n[Install]\nWantedBy=timers.target\n",
	}
}

func fixtureUnits(config contract.Config) map[string]string {
	units := Units(config)
	if config.Role == "boot-exit" {
		return map[string]string{contract.PeerUnit: units[contract.PeerUnit]}
	}
	delete(units, contract.PeerUnit)
	return units
}

func InstallUnits(ctx context.Context, config contract.Config) error {
	for name, content := range fixtureUnits(config) {
		if err := safeio.Create("/etc/systemd/system/"+name, []byte(content), 0644); err != nil {
			return err
		}
	}
	_, err := Manager(ctx, "daemon-reload")
	return err
}

func VerifyUnits(config contract.Config) error {
	for name, content := range fixtureUnits(config) {
		path := "/etc/systemd/system/" + name
		if err := safeio.CheckAccess(path, 0644, 0, 0); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != content {
			return fmt.Errorf("fixture unit ownership differs: %s", name)
		}
		entries, err := os.ReadDir(path + ".d")
		if err != nil && !os.IsNotExist(err) || len(entries) != 0 {
			return fmt.Errorf("unexpected fixture drop-in: %s", name)
		}
	}
	return nil
}

func UnitState(ctx context.Context, name string) (string, error) {
	raw, err := Manager(ctx, "show", "--property=ActiveState", "--value", name)
	return strings.TrimSpace(string(raw)), err
}
