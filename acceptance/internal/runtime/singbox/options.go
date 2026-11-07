package singbox

import (
	"strings"
	"vpn-script/internal/core/model"
)

func kind(id string) string {
	if strings.HasPrefix(id, "naive-") {
		return "naive"
	}
	if strings.HasPrefix(id, "hysteria2") {
		return "hysteria2"
	}
	if strings.HasPrefix(id, "snell") {
		return "snell"
	}
	if id == model.Shadowsocks {
		return "shadowsocks"
	}
	return id
}

func TLS(service model.Service, secret model.Secret, server bool) model.Object {
	value := model.Object{"enabled": true, "server_name": service.ServerName, "min_version": "1.3"}
	if server {
		value["certificate"], value["key"] = []string{secret.Certificate}, []string{secret.CertificateKey}
	} else if secret.TrustCertificate != "" {
		value["certificate"] = []string{secret.TrustCertificate}
	}
	if service.Protocol == "tuic" {
		value["alpn"] = []string{"h3"}
	}
	if kind(service.Protocol) == "naive" && !server {
		delete(value, "min_version")
	}
	return value
}

func protocolOptions(value model.Object, service model.Service, secret model.Secret, server bool) {
	switch kind(service.Protocol) {
	case "naive":
		if server {
			value["network"] = "tcp"
			if service.Protocol == "naive-h3" {
				value["network"] = "udp"
			}
		} else {
			value["username"], value["quic"], value["udp_over_tcp"] = "vpn", service.Protocol == "naive-h3", true
		}
	case "shadowsocks":
		value["method"], value["password"] = model.SSMethod, secret.Password
	case "snell":
		version := 6
		if service.Protocol == "snell-v5" {
			version = 4
			if server {
				version = 5
			}
		}
		value["version"], value["psk"] = version, secret.Password
	case "hysteria2":
		if service.Protocol != "hysteria2" {
			value["obfs"] = model.Object{"type": strings.TrimPrefix(service.Protocol, "hysteria2-"), "password": secret.ObfsPassword}
		}
	}
}
