package xray

import (
	"encoding/base64"
	"net"
	"net/url"
	"strconv"

	"vpn-script/internal/core/model"
)

func Client(proxy model.Object, port int) model.Xray {
	return model.Xray{Log: model.Object{"loglevel": "warning"},
		Inbounds: []model.Object{{"tag": "client-socks", "listen": "127.0.0.1", "port": port,
			"protocol": "socks", "settings": model.Object{"auth": "noauth", "udp": true}}},
		Outbounds: []model.Object{Block(), proxy},
		Routing:   model.Object{"rules": []model.Object{{"type": "field", "inboundTag": []string{"client-socks"}, "outboundTag": proxy["tag"]}}}}
}

func URI(topology *model.Topology, name string, secret model.Secret, route string) string {
	endpoint := topology.Services[name]
	address := net.JoinHostPort(topology.Nodes[endpoint.Node].Address, strconv.Itoa(endpoint.Service.Port))
	if secret.Protocol == "shadowsocks" {
		user := base64.RawURLEncoding.EncodeToString([]byte(SSMethod(endpoint.Service) + ":" + secret.Password))
		return "ss://" + user + "@" + address + "#" + url.PathEscape(route)
	}
	if secret.Protocol == model.Shadowsocks {
		return "ss://" + SSMethod(endpoint.Service) + ":" + url.QueryEscape(secret.Password) + "@" + address + "#" + url.PathEscape(route)
	}
	query := url.Values{"encryption": {"none"}, "security": {"reality"}, "type": {"tcp"},
		"flow": {"xtls-rprx-vision"}, "sni": {endpoint.Service.ServerName}, "fp": {"chrome"},
		"pbk": {secret.PublicKey}, "sid": {secret.ShortID}}
	value := url.URL{Scheme: "vless", Host: address, User: url.User(secret.UUID), RawQuery: query.Encode(), Fragment: route}
	return value.String()
}
