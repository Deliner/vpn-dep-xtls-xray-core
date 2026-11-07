package topology

import (
	"fmt"
	"net/netip"
	"strings"

	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
)

func nodes(topology *model.Topology) error {
	if len(topology.Nodes) == 0 {
		return fmt.Errorf("nodes must be nonempty")
	}
	addresses := map[string]bool{}
	for _, id := range dag.Keys(topology.Nodes) {
		node := topology.Nodes[id]
		if node == nil || validate.Name(id) != nil || validate.Host(node.Address) != nil {
			return fmt.Errorf("invalid node identifier/address")
		}
		address := strings.ToLower(node.Address)
		if parsed, err := netip.ParseAddr(address); err == nil {
			address = parsed.Unmap().String()
		}
		if addresses[address] {
			return fmt.Errorf("use one node ID per machine: duplicate address")
		}
		addresses[address] = true
		if err := normalizeNode(node); err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		if err := nodeServices(topology, id, node); err != nil {
			return err
		}
	}
	return nil
}

func normalizeNode(node *model.Node) error {
	if node.BridgePortStart == 0 {
		node.BridgePortStart = 40000
	}
	if node.BridgePortStart < 1024 || node.BridgePortStart > 65000 {
		return fmt.Errorf("bridge_port_start must be 1024..65000")
	}
	if err := validate.SSH(&node.SSH); err != nil {
		return err
	}
	if node.Firewall == "" {
		node.Firewall = "preserve"
	}
	if node.Firewall != "preserve" && node.Firewall != "ufw" {
		return fmt.Errorf("firewall must be preserve or ufw")
	}
	if len(node.Services) == 0 {
		return fmt.Errorf("services must be nonempty")
	}
	return nil
}

func nodeServices(topology *model.Topology, id string, node *model.Node) error {
	var previous []model.Service
	for _, name := range dag.Keys(node.Services) {
		value := node.Services[name]
		if err := validate.Name(name); err != nil {
			return err
		}
		if err := service(value, node.SSH); err != nil {
			return fmt.Errorf("%s.%s: %w", id, name, err)
		}
		for _, old := range previous {
			if collision(old, *value) {
				return fmt.Errorf("%s: listener port conflict", id)
			}
		}
		previous = append(previous, *value)
		topology.Services[id+"."+name] = model.Endpoint{Node: id, ID: name, Service: *value}
	}
	return nil
}
