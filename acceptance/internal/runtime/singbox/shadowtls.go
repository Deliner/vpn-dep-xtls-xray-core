package singbox

import (
	"net"
	"strconv"
	"vpn-script/internal/core/model"
)

func shadowInbounds(name string, service model.Service, secret model.Secret, innerPort int) []model.Object {
	host, rawPort, _ := net.SplitHostPort(service.Target)
	port, _ := strconv.Atoi(rawPort)
	outer := model.Object{"type": "shadowtls", "tag": name + "-tls", "listen": service.Listen, "listen_port": service.Port,
		"version": 3, "users": []model.Object{{"name": "vpn", "password": secret.ObfsPassword}}, "strict_mode": true,
		"handshake": model.Object{"server": host, "server_port": port}, "detour": name}
	// Omitted listen fields still create ephemeral sockets in sing-box. Own one
	// explicit loopback TCP listener; payload UDP travels inside its UoT stream.
	inner := model.Object{"type": "shadowsocks", "tag": name, "method": model.SSMethod, "password": secret.Password,
		"listen": "127.0.0.1", "listen_port": innerPort, "network": "tcp"}
	return []model.Object{outer, inner}
}

func shadowOutbounds(name, address string, service model.Service, secret model.Secret) []model.Object {
	outer := model.Object{"type": "shadowtls", "tag": "tls-" + name, "server": address, "server_port": service.Port,
		"version": 3, "password": secret.ObfsPassword, "tls": model.Object{"enabled": true, "server_name": service.ServerName,
			"utls": model.Object{"enabled": true, "fingerprint": "chrome"}}}
	if secret.TrustCertificate != "" {
		outer["tls"].(model.Object)["certificate"] = []string{secret.TrustCertificate}
	}
	inner := model.Object{"type": "shadowsocks", "tag": "to-" + name, "server": address, "server_port": service.Port,
		"method": model.SSMethod, "password": secret.Password, "detour": "tls-" + name, "udp_over_tcp": true}
	return []model.Object{inner, outer}
}
