package guest

import (
	"context"
	"fmt"
	"strings"

	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func Observe(ctx context.Context, config contract.Config, phase contract.Phase, values map[string]string) error {
	for _, unit := range []string{"nhop-client-boot.service", "systemd-networkd.service", "nhop-client.service", contract.CanaryUnit} {
		state, err := UnitState(ctx, unit)
		if err != nil {
			return err
		}
		values[unit] = state
		raw, err := Manager(ctx, "show", "--property=ActiveEnterTimestampMonotonic,ExecMainStartTimestampMonotonic,ExecMainExitTimestampMonotonic,Result,MainPID,After,Requires", unit)
		if err != nil {
			return err
		}
		values[unit+".properties"] = string(raw)
	}
	for key, args := range map[string][]string{"addresses": {"-j", "address"}, "routes": {"-j", "route"}} {
		raw, err := Command(ctx, "/usr/sbin/ip", args...)
		if err != nil {
			return err
		}
		values[key] = string(raw)
	}
	if phase.Name == "baseline" {
		return nil
	}
	if values[contract.CanaryUnit] != "active" {
		return fmt.Errorf("early NIC probe was not running")
	}
	if phase.Name == "guard-failure" {
		return failedGuard(values)
	}
	if values["nhop-client-boot.service"] != "active" || values["systemd-networkd.service"] != "active" || values["nhop-client.service"] != "active" {
		return fmt.Errorf("healthy boot did not start all protected dependencies")
	}
	return healthyTraffic(ctx, config, values)
}

func failedGuard(values map[string]string) error {
	if values["nhop-client-boot.service"] != "failed" || values["systemd-networkd.service"] == "active" || values["nhop-client.service"] == "active" {
		return fmt.Errorf("failed early protection did not prevent network/client startup")
	}
	if strings.Contains(values["addresses"], `"ifname":"nhop0"`) {
		return fmt.Errorf("failed protection nevertheless created a TUN")
	}
	return nil
}

func healthyTraffic(ctx context.Context, config contract.Config, values map[string]string) error {
	raw, err := Client(ctx, "diagnostics")
	values["diagnostics"] = string(raw)
	if err != nil {
		return err
	}
	traffic, err := Traffic(ctx, config, config.PeerIP)
	for key, value := range traffic {
		values[key] = value
	}
	if err != nil {
		return err
	}
	values["system_dns"], err = SystemDNS(ctx, config)
	return err
}
