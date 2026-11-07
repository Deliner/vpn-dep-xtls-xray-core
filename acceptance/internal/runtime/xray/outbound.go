package xray

import (
	"strings"
	"vpn-script/internal/core/model"
)

func Outbound(topology *model.Topology, name string, secret model.Secret) model.Object {
	endpoint := topology.Services[name]
	return Remote(name, topology.Nodes[endpoint.Node].Address, endpoint.Service, secret)
}

func Remote(name, address string, service model.Service, secret model.Secret) model.Object {
	value := model.Object{"tag": "to-" + name}
	server := model.Object{"address": address, "port": service.Port}
	switch {
	case strings.HasPrefix(service.Protocol, "shadowsocks"):
		value["protocol"] = "shadowsocks"
		server["method"], server["password"] = SSMethod(service), secret.Password
		value["settings"] = server
	case strings.HasPrefix(service.Protocol, "trojan"):
		value["protocol"] = "trojan"
		server["password"] = secret.Password
		value["settings"] = model.Object{"servers": []model.Object{server}}
	case strings.HasPrefix(service.Protocol, "masque"):
		value["protocol"] = "masque"
		server["remoteDNS"] = []string{"1.1.1.1"}
		value["settings"] = server
	default:
		value["protocol"] = userProtocol(service)
		server["users"] = []model.Object{userOutbound(service, secret)}
		value["settings"] = model.Object{"vnext": []model.Object{server}}
	}
	if !strings.HasPrefix(service.Protocol, "shadowsocks") {
		value["streamSettings"] = Stream(service, secret, false)
	}
	return value
}

func Block() model.Object { return model.Object{"tag": "block", "protocol": "blackhole"} }

// Exit enforces the UDP policy per packet, including destination changes within
// an existing session. A routing rule alone only checks the first destination.
func Exit() model.Object {
	return model.Object{"tag": "exit", "protocol": "freedom", "settings": model.Object{
		"finalRules": []model.Object{{"action": "block", "network": "udp", "port": "443"}},
	}}
}
