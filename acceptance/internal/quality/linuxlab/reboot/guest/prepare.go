package guest

import (
	"context"
	"fmt"
	"os"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

const certificatePath = "/usr/local/share/ca-certificates/nhop-client-reboot.crt"

func Prepare(ctx context.Context, config contract.Config) error {
	if config.Role == "boot-exit" {
		if err := originAddress(ctx, config, false); err != nil {
			return err
		}
		if err := InstallUnits(ctx, config); err != nil {
			return err
		}
		_, err := Manager(ctx, "start", contract.PeerUnit)
		return err
	}
	if err := certificate(ctx, false); err != nil {
		return err
	}
	if _, err := Client(ctx, "import", "--bundle", contract.Root+"/bundle", "--route", "main", "--output", contract.Root+"/profile.json"); err != nil {
		return err
	}
	if _, err := Client(ctx, "service-install", "--profile", contract.Root+"/profile.json", "--runtimes", contract.Root+"/runtimes.json",
		"--dns-ip", config.PeerIP, "--dns-name", "dns.example.invalid"); err != nil {
		return err
	}
	return InstallUnits(ctx, config)
}

func Arm(ctx context.Context, config contract.Config) error {
	if _, err := Phase(config); err != nil {
		return err
	}
	if err := VerifyUnits(config); err != nil {
		return err
	}
	if _, err := Manager(ctx, "enable", contract.CanaryUnit, contract.RecoveryTimer); err != nil {
		return err
	}
	if _, err := Client(ctx, "service-enable", "--network-manager", config.NetworkManager); err != nil {
		return err
	}
	for _, name := range []string{contract.CanaryUnit, contract.RecoveryTimer} {
		if _, err := Manager(ctx, "is-enabled", name); err != nil {
			return err
		}
	}
	return nil
}

func certificate(ctx context.Context, remove bool) error {
	data, err := os.ReadFile(contract.Root + "/dns.crt")
	if err != nil {
		return err
	}
	if !remove {
		err = safeio.Create(certificatePath, data, 0644)
	} else {
		if err := safeio.CheckAccess(certificatePath, 0644, 0, 0); err != nil {
			return err
		}
		actual, readError := os.ReadFile(certificatePath)
		if readError != nil || string(actual) != string(data) {
			return fmt.Errorf("fixture certificate changed; refusing removal")
		}
		err = os.Remove(certificatePath)
	}
	if err != nil {
		return err
	}
	_, err = Command(ctx, "/usr/sbin/update-ca-certificates")
	return err
}
