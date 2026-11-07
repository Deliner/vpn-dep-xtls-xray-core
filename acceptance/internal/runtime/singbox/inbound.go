package singbox

import (
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
)

func Inbounds(name string, service model.Service, secret model.Secret, innerPort int) []model.Object {
	if service.Protocol == "shadowtls-v3" {
		return shadowInbounds(name, service, secret, innerPort)
	}
	value := model.Object{"type": kind(service.Protocol), "tag": name, "listen": service.Listen, "listen_port": service.Port}
	if kind(service.Protocol) != "shadowsocks" && kind(service.Protocol) != "snell" {
		user := model.Object{"name": "vpn", "password": secret.Password}
		if kind(service.Protocol) == "naive" {
			delete(user, "name")
			user["username"] = "vpn"
		}
		if service.Protocol == "tuic" {
			user["uuid"] = secret.UUID
		}
		value["users"] = []model.Object{user}
	}
	spec, _ := protocol.Lookup(service.Protocol)
	if spec.TLS {
		value["tls"] = TLS(service, secret, true)
	}
	protocolOptions(value, service, secret, true)
	return []model.Object{value}
}
