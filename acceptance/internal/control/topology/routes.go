package topology

import (
	"fmt"
	"net/netip"

	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
)

func routes(topology *model.Topology) error {
	if len(topology.Routes) == 0 {
		return fmt.Errorf("routes must be nonempty")
	}
	for _, name := range dag.Keys(topology.Routes) {
		route := topology.Routes[name]
		if validate.Name(name) != nil || route == nil || len(route.Path) == 0 {
			return fmt.Errorf("invalid route name/path")
		}
		if err := normalizeRoute(route); err != nil {
			return err
		}
		if err := successors(topology, route.Path); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	if len(topology.Services) != len(topology.Successors) {
		return fmt.Errorf("services without an explicit route")
	}
	return nil
}

func normalizeRoute(route *model.Route) error {
	if route.Client.SOCKSPort == 0 {
		route.Client.SOCKSPort = 10808
	}
	if err := validate.Port(route.Client.SOCKSPort); err != nil {
		return err
	}
	if route.ExpectedExitIP != "" {
		address, err := netip.ParseAddr(route.ExpectedExitIP)
		if err != nil || address.Zone() != "" {
			return fmt.Errorf("expected_exit_ip must be an IP literal without scope zone")
		}
		route.ExpectedExitIP = address.Unmap().String()
	}
	return nil
}

func successors(topology *model.Topology, path []string) error {
	seen := map[string]bool{}
	for index, from := range path {
		if _, exists := topology.Services[from]; !exists || seen[from] {
			return fmt.Errorf("unknown service or route cycle")
		}
		seen[from] = true
		to := ""
		if index+1 < len(path) {
			to = path[index+1]
		}
		if old, exists := topology.Successors[from]; exists && old != to {
			return fmt.Errorf("ambiguous next hop for %s: use separate listeners", from)
		}
		topology.Successors[from] = to
	}
	return nil
}
