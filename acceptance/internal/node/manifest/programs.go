package manifest

import (
	"fmt"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/runtime/engines"
)

func programListeners(data model.NodeConfig) ([]model.Listener, error) {
	var result []model.Listener
	seen := map[string]bool{}
	for _, program := range engines.Programs(data) {
		if seen[program.Name] || (program.Name != "xray" && program.Name != "sing-box") || !reflect.DeepEqual(program.Runtime, engines.Runtime(program.Name)) {
			return nil, fmt.Errorf("unknown, duplicate or unpinned engine")
		}
		seen[program.Name] = true
		program.Config = engines.Object(program.Config)
		if err := blockedDefault(program); err != nil {
			return nil, err
		}
		program.Config = engines.Object(program.Config)
		raw, _ := program.Config["inbounds"].([]any)
		for _, inbound := range raw {
			object, ok := inbound.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid inbound")
			}
			listeners, err := engineListeners(program.Name, object)
			if err != nil {
				return nil, err
			}
			result = append(result, listeners...)
		}
	}
	return result, nil
}

func blockedDefault(program model.Program) error {
	if program.Name == "xray" {
		outs, _ := program.Config["outbounds"].([]any)
		if len(outs) == 0 {
			return fmt.Errorf("missing outbounds")
		}
		first, _ := outs[0].(map[string]any)
		if first["protocol"] != "blackhole" || first["tag"] != "block" {
			return fmt.Errorf("default outbound must be block")
		}
		return nil
	}
	route, _ := program.Config["route"].(map[string]any)
	rules, _ := route["rules"].([]any)
	if len(rules) == 0 {
		return fmt.Errorf("missing fail-closed route")
	}
	last, _ := rules[len(rules)-1].(map[string]any)
	if len(last) != 1 || last["action"] != "reject" || route["final"] != nil {
		return fmt.Errorf("unmatched routes must reject")
	}
	return nil
}

func engineListeners(engine string, value model.Object) ([]model.Listener, error) {
	if engine == "xray" {
		return inboundListeners(value)
	}
	address, _ := value["listen"].(string)
	tag, _ := value["tag"].(string)
	port, err := strconv.Atoi(fmt.Sprint(value["listen_port"]))
	if err != nil || validate.Port(port) != nil || tag == "" {
		return nil, fmt.Errorf("invalid engine listener")
	}
	if _, err := netip.ParseAddr(address); err != nil {
		return nil, fmt.Errorf("listen must be IP literal")
	}
	network, err := singBoxNetwork(value)
	if err != nil {
		return nil, err
	}
	var result []model.Listener
	for _, network := range strings.Split(network, ",") {
		result = append(result, model.Listener{Tag: tag, Address: address, Port: port, Network: network})
	}
	return result, nil
}

func singBoxNetwork(value model.Object) (string, error) {
	kind, _ := value["type"].(string)
	switch kind {
	case "tuic", "hysteria2":
		return "udp", nil
	case "shadowsocks", "naive":
		if kind == "shadowsocks" && value["network"] == nil {
			return "tcp,udp", nil
		}
		network, _ := value["network"].(string)
		if network != "tcp" && network != "udp" {
			return "", fmt.Errorf("listener network must be tcp or udp")
		}
		return network, nil
	case "anytls", "snell", "shadowtls":
		return "tcp", nil
	default:
		return "", fmt.Errorf("unsupported engine inbound %s", kind)
	}
}

func inboundListeners(value model.Object) ([]model.Listener, error) {
	protocol, _ := value["protocol"].(string)
	address, _ := value["listen"].(string)
	tag, _ := value["tag"].(string)
	number, err := strconv.Atoi(fmt.Sprint(value["port"]))
	if err != nil || validate.Port(number) != nil || tag == "" {
		return nil, fmt.Errorf("invalid inbound listener")
	}
	if _, err := netip.ParseAddr(address); err != nil {
		return nil, fmt.Errorf("inbound listen must be an IP literal")
	}
	if protocol != "vless" && protocol != "vmess" && protocol != "shadowsocks" && protocol != "trojan" && protocol != "masque" {
		return nil, fmt.Errorf("unsupported inbound protocol")
	}
	result := []model.Listener{{Tag: tag, Address: address, Port: number, Network: "tcp"}}
	if protocol == "shadowsocks" || protocol == "masque" {
		result = append(result, model.Listener{Tag: tag, Address: address, Port: number, Network: "udp"})
	}
	return result, nil
}

func validateListeners(data model.NodeConfig) error {
	listeners, err := programListeners(data)
	if err != nil {
		return err
	}
	actual, err := listenerSet(listeners)
	if err != nil {
		return err
	}
	if len(actual) == 0 || len(actual) != len(data.Listeners) {
		return fmt.Errorf("listener metadata mismatch")
	}
	for _, listener := range data.Listeners {
		address, err := netip.ParseAddr(listener.Address)
		if err != nil {
			return err
		}
		if listener.Internal && !address.IsLoopback() {
			return fmt.Errorf("internal listener must bind loopback")
		}
		listener.Internal = false
		if !actual[listener] {
			return fmt.Errorf("listener metadata mismatch")
		}
		delete(actual, listener)
	}
	return nil
}

func listenerSet(listeners []model.Listener) (map[model.Listener]bool, error) {
	actual := map[model.Listener]bool{}
	for _, listener := range listeners {
		for old := range actual {
			if validate.SocketsOverlap(old, listener) {
				return nil, fmt.Errorf("conflicting inbound sockets: %s and %s on %d/%s", old.Tag, listener.Tag, listener.Port, listener.Network)
			}
		}
		actual[listener] = true
	}
	return actual, nil
}
