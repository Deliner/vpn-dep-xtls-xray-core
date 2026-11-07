package singbox

import (
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
)

func Outbounds(name, address string, service model.Service, secret model.Secret) []model.Object {
	if service.Protocol == "shadowtls-v3" {
		return shadowOutbounds(name, address, service, secret)
	}
	value := model.Object{"type": kind(service.Protocol), "tag": "to-" + name, "server": address, "server_port": service.Port}
	if kind(service.Protocol) != "snell" {
		value["password"] = secret.Password
	}
	if service.Protocol == "tuic" {
		value["uuid"] = secret.UUID
		value["zero_rtt_handshake"] = false
	}
	spec, _ := protocol.Lookup(service.Protocol)
	if spec.TLS {
		value["tls"] = TLS(service, secret, false)
	}
	protocolOptions(value, service, secret, false)
	return []model.Object{value}
}
