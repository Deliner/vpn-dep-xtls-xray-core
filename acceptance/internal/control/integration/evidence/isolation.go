package evidence

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
)

// PrivatePortNamespace protects the deliberately exhausted-port regression.
// Its sysctl belongs to a separate network namespace inside the owned Linux VM.
func PrivatePortNamespace() error {
	if err := PrivateNetworkNamespace(); err != nil {
		return err
	}
	raw, err := os.ReadFile("/proc/sys/net/ipv4/ip_local_port_range")
	if err != nil || strings.Join(strings.Fields(string(raw)), " ") != "45000 45031" {
		return fmt.Errorf("requires the isolated 32-port ephemeral range")
	}
	return nil
}

// PrivateNetworkNamespace rejects the host and the owned VM's main namespace.
func PrivateNetworkNamespace() error {
	host, err := os.Hostname()
	if err != nil || runtime.GOOS != "linux" || os.Geteuid() != 0 || host != "lima-payload" {
		return fmt.Errorf("requires root in the private Linux VM")
	}
	current, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		return err
	}
	initial, err := os.Readlink("/proc/1/ns/net")
	if err != nil || current == initial {
		return fmt.Errorf("requires a separate network namespace")
	}
	return nil
}

// PrivateRoutingNamespace makes the synthetic target locally reachable so a
// mistaken direct route cannot pass the rejection test merely by being unroutable.
func PrivateRoutingNamespace() error {
	if err := PrivateNetworkNamespace(); err != nil {
		return err
	}
	interfaces, err := net.Interfaces()
	if err != nil || len(interfaces) != 1 || interfaces[0].Name != "lo" {
		return fmt.Errorf("routing canary requires a loopback-only namespace")
	}
	addresses, err := interfaces[0].Addrs()
	if err != nil {
		return err
	}
	for _, address := range addresses {
		if address.String() == "198.18.0.2/32" {
			return nil
		}
	}
	return fmt.Errorf("routing canary requires 198.18.0.2/32 on isolated loopback")
}
