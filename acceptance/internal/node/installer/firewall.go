package installer

import (
	"fmt"
	"net/netip"
	"os/exec"
	"strings"

	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

func UFWProfile(data model.NodeConfig) string {
	ports := map[string]bool{}
	for _, listener := range data.Listeners {
		address, _ := netip.ParseAddr(listener.Address)
		if listener.Internal || address.IsLoopback() {
			continue
		}
		ports[fmt.Sprintf("%d/%s", listener.Port, listener.Network)] = true
	}
	return marker + "\n[VPN-NHOP]\ntitle=N-hop VPN\ndescription=Managed VPN listeners\nports=" + strings.Join(dag.Keys(ports), "|") + "\n"
}

func (i *Installer) firewallPreflight(data model.NodeConfig) error {
	if data.Firewall != "ufw" {
		return nil
	}
	if _, err := exec.LookPath("ufw"); err != nil {
		return fmt.Errorf("firewall=ufw requires installed, already active UFW")
	}
	raw, err := i.command("ufw", "status")
	if err != nil || !strings.Contains(string(raw), "Status: active") {
		return fmt.Errorf("firewall=ufw requires already active UFW; bootstrap SSH separately")
	}
	profile, err := optionalFile(i.Paths.Firewall)
	if err != nil {
		return err
	}
	if profile != nil && !strings.HasPrefix(string(profile), marker) {
		return fmt.Errorf("refusing to overwrite an unmanaged UFW profile")
	}
	return nil
}

func (i *Installer) applyFirewall(data model.NodeConfig) error {
	if data.Firewall != "ufw" {
		return nil
	}
	if err := safeio.Atomic(i.Paths.Firewall, []byte(UFWProfile(data)), 0644); err != nil {
		return err
	}
	if _, err := i.command("ufw", "app", "update", "VPN-NHOP"); err != nil {
		return err
	}
	_, err := i.command("ufw", "allow", "VPN-NHOP")
	return err
}

func (i *Installer) restoreFirewall(previous []byte, changed bool) error {
	if !changed {
		return nil
	}
	if previous == nil {
		current, err := optionalFile(i.Paths.Firewall)
		if err != nil {
			return err
		}
		if current != nil {
			if _, err := i.command("ufw", "--force", "delete", "allow", "VPN-NHOP"); err != nil {
				return err
			}
		}
		return restoreFile(i.Paths.Firewall, nil, 0644)
	}
	if err := safeio.Atomic(i.Paths.Firewall, previous, 0644); err != nil {
		return err
	}
	_, err := i.command("ufw", "app", "update", "VPN-NHOP")
	return err
}
