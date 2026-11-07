package xray

import (
	"strings"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
)

func Inbound(name string, service model.Service, secret model.Secret) model.Object {
	value := model.Object{"tag": name, "listen": service.Listen, "port": service.Port}
	switch {
	case strings.HasPrefix(service.Protocol, "shadowsocks"):
		value["protocol"] = "shadowsocks"
		value["settings"] = model.Object{"method": SSMethod(service), "password": secret.Password, "network": "tcp,udp"}
	case strings.HasPrefix(service.Protocol, "trojan"):
		value["protocol"] = "trojan"
		value["settings"] = model.Object{"clients": []model.Object{{"password": secret.Password}}}
	case strings.HasPrefix(service.Protocol, "masque"):
		value["protocol"] = "masque"
		value["settings"] = model.Object{"users": []model.Object{{"email": "vpn", "pass": secret.Password}}, "address": []string{"10.234.0.1/24"}, "mtu": 1280}
	default:
		value["protocol"], value["settings"] = userProtocol(service), userInbound(service, secret)
	}
	if !strings.HasPrefix(service.Protocol, "shadowsocks") {
		value["streamSettings"] = Stream(service, secret, true)
	}
	return value
}

func Listeners(name string, service model.Service) []model.Listener {
	spec, _ := protocol.Lookup(service.Protocol)
	var values []model.Listener
	for _, network := range strings.Split(spec.Network, ",") {
		values = append(values, model.Listener{Tag: name, Address: service.Listen, Port: service.Port, Network: network})
	}
	return values
}

func SSMethod(service model.Service) string {
	if service.Protocol == "shadowsocks" {
		return "chacha20-ietf-poly1305"
	}
	return model.SSMethod
}
