package engines

import (
	"vpn-script/internal/core/model"
	"vpn-script/internal/runtime/singbox"
	"vpn-script/internal/runtime/xray"
)

func (g *graph) exit(engine, name string, secret model.Secret) error {
	if engine == "sing-box" {
		outbound, err := g.guardedUDP(secret)
		if err != nil {
			return err
		}
		rule := singbox.Rule(name, outbound)
		rule["network"] = "udp"
		g.rules[engine] = append(g.rules[engine], rule)
	}
	g.route(engine, name, g.outbound(engine, "", "", model.Service{}, model.Secret{}))
	return nil
}

// One authenticated loopback bridge per node supplies per-packet egress checks
// for sing-box, whose route rules select an entire UDP association. TCP remains
// on its existing direct outbound. Internal listeners use the normal allocator,
// manifest and readiness checks; no firewall or privileged sidecar is needed.
func (g *graph) guardedUDP(secret model.Secret) (string, error) {
	if g.udpExit != "" {
		return g.udpExit, nil
	}
	port, err := g.port()
	if err != nil {
		return "", err
	}
	const name = "bridge-udp-exit"
	bridge := model.Service{Protocol: model.Shadowsocks, Listen: "127.0.0.1", Port: port}
	key := model.Secret{Protocol: model.Shadowsocks, Password: secret.BridgePassword}
	if err := g.inbound("xray", name, bridge, key, true); err != nil {
		return "", err
	}
	if g.outbounds["xray"] == nil {
		g.outbounds["xray"] = map[string]model.Object{}
	}
	guard := xray.Exit()
	guard["tag"] = "exit-udp"
	settings := guard["settings"].(model.Object)
	// Preserve sing-box's existing destination policy after blocking UDP/443;
	// Xray's implicit private-address policy otherwise changes that contract.
	settings["finalRules"] = append(settings["finalRules"].([]model.Object), model.Object{"action": "allow", "network": "udp"})
	g.outbounds["xray"]["exit-udp"] = guard
	g.rules["xray"] = append(g.rules["xray"], model.Object{"type": "field", "inboundTag": []string{name}, "network": "udp", "outboundTag": "exit-udp"})
	g.udpExit = g.outbound("sing-box", name, "127.0.0.1", bridge, key)
	return g.udpExit, nil
}
