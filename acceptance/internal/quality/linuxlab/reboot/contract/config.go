// Package contract defines the private, two-VM reboot acceptance inputs.
package contract

import (
	"encoding/hex"
	"fmt"
	"net/netip"

	"vpn-script/internal/platform/safeio"
)

const Root = "/opt/nhop-client-reboot"

// OriginIP is an address owned only by the exit VM's loopback device.
// It differs from the VPN entry IP, whose routing exception deliberately bypasses TUN.
const OriginIP = "198.18.0.2"
const PeerUnit = "nhop-reboot-peer.service"
const CanaryUnit = "nhop-reboot-canary.service"
const RecoveryUnit = "nhop-reboot-recover.service"
const RecoveryTimer = "nhop-reboot-recover.timer"

type Config struct {
	SchemaVersion  int    `json:"schema_version"`
	Token          string `json:"token"`
	Role           string `json:"role"`
	ClientIP       string `json:"client_ip"`
	PeerIP         string `json:"peer_ip"`
	Interface      string `json:"interface"`
	NetworkManager string `json:"network_manager"`
}

type Phase struct {
	Token        string `json:"token"`
	Name         string `json:"name"`
	BeforeBootID string `json:"before_boot_id"`
}

func (c Config) Validate() error {
	token, err := hex.DecodeString(c.Token)
	if err != nil || len(token) != 32 || c.SchemaVersion != 2 {
		return fmt.Errorf("invalid reboot lab identity")
	}
	if c.Role != "boot-client" && c.Role != "boot-exit" || c.Interface != "eth0" || c.ClientIP == c.PeerIP {
		return fmt.Errorf("unexpected reboot lab role or interface")
	}
	if c.NetworkManager != "systemd-networkd.service" && c.NetworkManager != "NetworkManager.service" {
		return fmt.Errorf("unsupported reboot network manager")
	}
	for _, text := range []string{c.ClientIP, c.PeerIP} {
		address, err := netip.ParseAddr(text)
		if err != nil || !netip.MustParsePrefix("192.168.125.0/24").Contains(address) || !address.IsGlobalUnicast() {
			return fmt.Errorf("reboot lab endpoint is outside its owned network")
		}
	}
	return nil
}

func Load() (Config, error) {
	var c Config
	if err := safeio.LoadJSON(Root+"/config.json", &c); err != nil {
		return c, err
	}
	return c, c.Validate()
}
