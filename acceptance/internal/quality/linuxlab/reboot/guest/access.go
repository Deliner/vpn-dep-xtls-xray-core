// Package guest owns privileged acceptance operations in the dedicated reboot VMs.
package guest

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func Require(role string) (contract.Config, error) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 || os.Getegid() != 0 || os.Getenv("NHOP_REBOOT_GUEST") != "owned-vm" {
		return contract.Config{}, fmt.Errorf("reboot fixture requires explicit Linux root in its dedicated VM")
	}
	name, err := os.Hostname()
	if err != nil || name != "lima-"+role {
		return contract.Config{}, fmt.Errorf("unexpected reboot fixture hostname")
	}
	for _, kind := range []string{"net", "mnt"} {
		current, err := os.Readlink("/proc/self/ns/" + kind)
		initial, other := os.Readlink("/proc/1/ns/" + kind)
		if err != nil || other != nil || current != initial {
			return contract.Config{}, fmt.Errorf("real reboot fixture must use the guest root namespaces")
		}
	}
	manager, err := os.ReadFile("/proc/1/comm")
	if err != nil || strings.TrimSpace(string(manager)) != "systemd" {
		return contract.Config{}, fmt.Errorf("reboot fixture requires systemd as PID 1")
	}
	if err := privateInputs(); err != nil {
		return contract.Config{}, err
	}
	c, err := contract.Load()
	if err == nil && c.Role != role {
		err = fmt.Errorf("reboot fixture configuration belongs to another VM")
	}
	return c, err
}

func privateInputs() error {
	if err := safeio.ProtectedParent(contract.Root+"/config.json", 0); err != nil {
		return err
	}
	if err := safeio.CheckAccess(contract.Root, os.ModeDir|0700, 0, 0); err != nil {
		return err
	}
	return safeio.CheckAccess(contract.Root+"/config.json", 0600, 0, 0)
}

func Phase(config contract.Config) (contract.Phase, error) {
	var phase contract.Phase
	if err := safeio.ProtectedParent(contract.Root+"/phase.json", 0); err != nil {
		return phase, err
	}
	if err := safeio.CheckAccess(contract.Root+"/phase.json", 0600, 0, 0); err != nil {
		return phase, err
	}
	if err := safeio.LoadJSON(contract.Root+"/phase.json", &phase); err != nil {
		return phase, err
	}
	if phase.Token != config.Token || phase.BeforeBootID == "" || phase.Name != "healthy" && phase.Name != "guard-failure" && phase.Name != "baseline" {
		return phase, fmt.Errorf("invalid reboot phase")
	}
	return phase, nil
}

func BootID() (string, error) {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(data))
	if len(id) != 36 || strings.Count(id, "-") != 4 {
		return "", fmt.Errorf("invalid kernel boot identity")
	}
	return id, nil
}
