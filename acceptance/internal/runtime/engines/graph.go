package engines

import (
	"fmt"
	"strings"
	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/runtime/singbox"
	"vpn-script/internal/runtime/xray"
)

func newGraph(node *model.Node) *graph {
	g := &graph{inbounds: map[string][]model.Object{}, outbounds: map[string]map[string]model.Object{},
		rules: map[string][]model.Object{}, used: map[int]bool{node.SSH.Port: true}, nextPort: node.BridgePortStart}
	if g.nextPort == 0 {
		g.nextPort = 40000
	}
	for _, service := range node.Services {
		g.used[service.Port] = true
	}
	for _, port := range node.SSH.ClosedPorts {
		g.used[port] = true
	}
	return g
}

func (g *graph) port() (int, error) {
	for g.nextPort <= 65535 {
		port := g.nextPort
		g.nextPort++
		if !g.used[port] {
			g.used[port] = true
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free internal listener ports in configured range")
}

func (g *graph) inbound(engine, name string, service model.Service, secret model.Secret, internal bool) error {
	innerPort := 0
	if service.Protocol == "shadowtls-v3" {
		var err error
		innerPort, err = g.port()
		if err != nil {
			return err
		}
		g.listeners = append(g.listeners, model.Listener{Tag: name, Address: "127.0.0.1", Port: innerPort, Network: "tcp", Internal: true})
	}
	if engine == "xray" {
		g.inbounds[engine] = append(g.inbounds[engine], xray.Inbound(name, service, secret))
	} else {
		g.inbounds[engine] = append(g.inbounds[engine], singbox.Inbounds(name, service, secret, innerPort)...)
	}
	for _, listener := range xray.Listeners(name, service) {
		listener.Internal = internal
		if service.Protocol == "shadowtls-v3" {
			listener.Tag += "-tls"
		}
		g.listeners = append(g.listeners, listener)
	}
	return nil
}

func (g *graph) outbound(engine, name, address string, service model.Service, secret model.Secret) string {
	if g.outbounds[engine] == nil {
		g.outbounds[engine] = map[string]model.Object{}
	}
	values := []model.Object{}
	if name == "" {
		value := model.Object{"tag": "exit", "type": "direct"}
		if engine == "xray" {
			value = xray.Exit()
		}
		values = append(values, value)
	} else if engine == "xray" {
		values = append(values, xray.Remote(name, address, service, secret))
	} else {
		values = append(values, singbox.Outbounds(name, address, service, secret)...)
	}
	for _, value := range values {
		g.outbounds[engine][value["tag"].(string)] = value
	}
	return values[0]["tag"].(string)
}

func (g *graph) route(engine, inbound, outbound string) {
	rule := singbox.Rule(inbound, outbound)
	if engine == "xray" {
		rule = model.Object{"type": "field", "inboundTag": []string{inbound}, "network": "tcp,udp", "outboundTag": outbound}
	}
	g.rules[engine] = append(g.rules[engine], rule)
}

func (g *graph) programs() []model.Program {
	var result []model.Program
	for _, engine := range dag.Keys(g.inbounds) {
		var outs []model.Object
		for _, tag := range dag.Keys(g.outbounds[engine]) {
			outs = append(outs, g.outbounds[engine][tag])
		}
		rules := append([]model.Object{{"network": "udp", "port": 443, "action": "reject"}}, g.rules[engine]...)
		config := singbox.Config(g.inbounds[engine], outs, rules)
		if strings.EqualFold(engine, "xray") {
			rules = append([]model.Object{{"type": "field", "network": "udp", "port": "443", "outboundTag": "block"}}, g.rules[engine]...)
			config = Object(model.Xray{Log: model.Object{"loglevel": "warning"}, Inbounds: g.inbounds[engine],
				Outbounds: append([]model.Object{xray.Block()}, outs...), Routing: model.Object{"rules": rules}})
		}
		result = append(result, model.Program{Name: engine, Runtime: Runtime(engine), Config: config})
	}
	return result
}
