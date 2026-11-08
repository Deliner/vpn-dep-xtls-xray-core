package guest

import (
	"context"
	"os"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

// Restore stops the probe before explicitly releasing the production protection.
// It runs locally, so failed DHCP/SSH cannot prevent bounded recovery.
func Restore(ctx context.Context, config contract.Config, phase contract.Phase) error {
	if err := VerifyUnits(config); err != nil {
		return err
	}
	if _, err := Manager(ctx, "stop", contract.CanaryUnit); err != nil {
		return err
	}
	if _, err := Manager(ctx, "stop", "nhop-client.service"); err != nil {
		return err
	}
	if phase.Name == "guard-failure" {
		if err := safeio.CheckAccess("/run/nhop-client", os.ModeDir|0755, 0, 0); err == nil {
			if err := os.Chmod("/run/nhop-client", 0711); err != nil {
				return err
			}
		}
	}
	if _, err := Client(ctx, "stop"); err != nil {
		return err
	}
	if err := settleClient(ctx); err != nil {
		return err
	}
	if _, err := Client(ctx, "service-disable"); err != nil {
		return err
	}
	if _, err := Manager(ctx, "disable", contract.CanaryUnit, contract.RecoveryTimer); err != nil {
		return err
	}
	_, err := Manager(ctx, "start", "systemd-networkd.service")
	return err
}

// SIGTERM deliberately reports failure while retaining the firewall. Once the
// owned service is stopped and explicit stop succeeded, clear that failed state.
func settleClient(ctx context.Context) error {
	state, err := UnitState(ctx, "nhop-client.service")
	if err != nil || state != "failed" {
		return err
	}
	_, err = Manager(ctx, "reset-failed", "nhop-client.service")
	return err
}

func Clean(ctx context.Context, config contract.Config) error {
	if err := VerifyUnits(config); err != nil {
		return err
	}
	if config.Role == "boot-client" {
		phase, err := Phase(config)
		if err != nil {
			return err
		}
		if err := Restore(ctx, config, phase); err != nil {
			return err
		}
		if _, err := Manager(ctx, "stop", contract.RecoveryTimer, contract.RecoveryUnit); err != nil {
			return err
		}
		if _, err := Client(ctx, "service-remove"); err != nil {
			return err
		}
		if err := certificate(ctx, true); err != nil {
			return err
		}
	} else {
		if _, err := Manager(ctx, "stop", contract.PeerUnit); err != nil {
			return err
		}
		if err := originAddress(ctx, config, true); err != nil {
			return err
		}
	}
	for name := range fixtureUnits(config) {
		if err := os.Remove("/etc/systemd/system/" + name); err != nil {
			return err
		}
	}
	_, err := Manager(ctx, "daemon-reload")
	return err
}
