package reboot

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func (f *fixture) identify(t *testing.T) {
	t.Helper()
	addresses := map[string]string{}
	for _, role := range roles {
		if strings.TrimSpace(string(f.remote(t, role, "hostname"))) != "lima-"+role {
			t.Fatal("unexpected VM hostname")
		}
		f.boots[role] = strings.TrimSpace(string(f.save(t, role, "/proc/sys/kernel/random/boot_id", "initial-boot-id.txt")))
		addresses[role] = f.address(t, role)
		for name, command := range map[string][]string{"kernel": {"uname", "-a"}, "systemd": {"systemctl", "--version"}, "addresses": {"ip", "-j", "address"}} {
			require(t, safeio.Atomic(filepath.Join(f.root, role+"-initial-"+name+".txt"), f.remote(t, role, command...), 0600))
		}
	}
	if len(f.boots["boot-client"]) != 36 || f.boots["boot-client"] == f.boots["boot-exit"] {
		t.Fatal("two independently booted kernels are required")
	}
	var token [32]byte
	_, err := rand.Read(token[:])
	require(t, err)
	for _, role := range roles {
		manager := "systemd-networkd.service"
		if role == "boot-client" {
			manager = os.Getenv("NHOP_REBOOT_NETWORK_MANAGER")
		}
		config := contract.Config{SchemaVersion: 2, Role: role, Token: hex.EncodeToString(token[:]), ClientIP: addresses["boot-client"], PeerIP: addresses["boot-exit"], Interface: "eth0", NetworkManager: manager}
		require(t, config.Validate())
		f.config[role] = config
	}
}

func (f *fixture) address(t *testing.T, role string) string {
	t.Helper()
	type address struct {
		Local string `json:"local"`
	}
	type device struct {
		Addresses []address `json:"addr_info"`
	}
	devices := decode[[]device](t, f.remote(t, role, "ip", "-j", "-4", "address", "show", "dev", "eth0"))
	if len(devices) != 1 || len(devices[0].Addresses) != 1 {
		t.Fatal("expected exactly one owned DHCP IPv4 address")
	}
	return devices[0].Addresses[0].Local
}

func (f *fixture) stage(t *testing.T, role string) {
	t.Helper()
	f.remote(t, role, "test", "!", "-e", contract.Root)
	f.remote(t, role, "install", "-d", "-m", "0700", contract.Root, contract.Root+"/bin", contract.Root+"/bundle", contract.Root+"/bundle/clients")
	f.putJSON(t, role, "config.json", f.config[role])
	names := []string{"reboot.test", "bin/xray", "dns.crt"}
	if role == "boot-exit" {
		names = append(names, "server.json", "dns.key")
	} else {
		names = append(names, "bin/nhop-client", "bin/sing-box", "bin/libcronet.so", "runtimes.json", "bundle/client-profiles.json", "bundle/clients/main.json")
	}
	for _, name := range names {
		mode := "0600"
		if name == "reboot.test" || strings.HasPrefix(name, "bin/") {
			mode = "0700"
		}
		f.put(t, role, filepath.Join(f.root, name), name, mode)
	}
	f.guest(t, role, "TestRebootPrepare")
	t.Cleanup(func() { f.clean(t, role) })
}

func (f *fixture) phase(t *testing.T, name string) {
	t.Helper()
	boot := strings.TrimSpace(string(f.remote(t, "boot-client", "cat", "/proc/sys/kernel/random/boot_id")))
	f.putJSON(t, "boot-client", "phase.json", contract.Phase{Token: f.config["boot-client"].Token, Name: name, BeforeBootID: boot})
	f.boots["boot-client"] = boot
}
