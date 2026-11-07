package singbox

import "vpn-script/internal/core/model"

func Config(inbounds, outbounds, rules []model.Object) model.Object {
	// An unmatched inbound is rejected. Explicit final=exit would leak broken routes.
	rules = append(rules, model.Object{"action": "reject"})
	return model.Object{"log": model.Object{"level": "warn"}, "inbounds": inbounds, "outbounds": outbounds,
		"dns":   model.Object{"servers": []model.Object{{"type": "local", "tag": "system"}}},
		"route": model.Object{"rules": rules, "default_domain_resolver": "system"}}
}

func Rule(inbound, outbound string) model.Object {
	return model.Object{"inbound": []string{inbound}, "action": "route", "outbound": outbound}
}

func Client(outbounds []model.Object, port int) model.Object {
	return Config([]model.Object{{"type": "socks", "tag": "client-socks", "listen": "127.0.0.1", "listen_port": port}},
		outbounds, []model.Object{Rule("client-socks", outbounds[0]["tag"].(string))})
}
