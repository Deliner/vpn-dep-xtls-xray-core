// Package routing owns the client's reserved route table and explicit policy
// rules independently of the TUN engine's shutdown hooks.
package routing

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"runtime"
	"strconv"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/platform/process"
)

// New checks that neither our table nor reserved priorities are already used.
// No routing mutation occurs until the caller has installed the kill switch.
func New(ctx context.Context, entry, interfaceName string, ipv6 bool) (*Manager, error) {
	m, err := Recorded(entry, interfaceName, ipv6)
	if err != nil {
		return nil, err
	}
	return m, m.vacant(ctx)
}

// Recorded reconstructs selectors from a trusted private session journal for
// explicit recovery. Restore still requires absent TUN routes and exact rules.
func Recorded(entry, interfaceName string, ipv6 bool) (*Manager, error) {
	address, err := netip.ParseAddr(entry)
	if err != nil || runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return nil, fmt.Errorf("route ownership requires Linux root and an explicit entry IP")
	}
	m := &Manager{run: process.Run, interfaceName: interfaceName}
	for _, family := range []string{"-4", "-6"} {
		if family == "-6" && !ipv6 {
			continue
		}
		if address.Is4() == (family == "-4") {
			m.rules = append(m.rules, rule{family, netip.PrefixFrom(address, address.BitLen()).String(), "main", network.RouteIndex})
		}
		m.rules = append(m.rules, rule{family, "", strconv.Itoa(network.RouteIndex), network.RouteIndex + 1})
	}
	return m, nil
}

func (r rule) args(operation string) []string {
	args := []string{"ip", r.family, "rule", operation, "pref", strconv.Itoa(r.priority), "protocol", ownerProtocol, "table", r.table}
	if r.destination != "" {
		args = append(args, "to", r.destination)
	}
	return args
}
