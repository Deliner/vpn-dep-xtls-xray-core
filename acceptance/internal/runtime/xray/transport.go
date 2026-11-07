package xray

import (
	"strings"
	"vpn-script/internal/core/model"
)

func Stream(service model.Service, secret model.Secret, server bool) model.Object {
	if service.Protocol == "vless-encryption" {
		return model.Object{"network": "tcp", "security": "none"}
	}
	if service.Protocol == model.Reality {
		return realityStream(service, secret, server)
	}
	value := model.Object{"network": "tcp", "security": "tls", "tlsSettings": tlsSettings(service, secret, server)}
	if strings.HasSuffix(service.Protocol, "-ws") {
		value["network"], value["wsSettings"] = "ws", model.Object{"path": service.Path}
	}
	if service.Protocol == "vless-xhttp" {
		value["network"], value["xhttpSettings"] = "xhttp", model.Object{"path": service.Path, "mode": "packet-up"}
	}
	if strings.HasPrefix(service.Protocol, "masque-") {
		settings := model.Object{}
		if !server {
			settings["user"], settings["pass"] = "vpn", secret.Password
		}
		value["network"], value["masqueSettings"] = "masque", settings
	}
	return value
}

func realityStream(service model.Service, secret model.Secret, server bool) model.Object {
	settings := model.Object{"serverName": service.ServerName, "fingerprint": "chrome", "publicKey": secret.PublicKey, "shortId": secret.ShortID}
	if server {
		settings = model.Object{"dest": service.Target, "serverNames": []string{service.ServerName}, "privateKey": secret.PrivateKey, "shortIds": []string{secret.ShortID}}
	}
	return model.Object{"network": "tcp", "security": "reality", "realitySettings": settings}
}

func tlsSettings(service model.Service, secret model.Secret, server bool) model.Object {
	value := model.Object{"serverName": service.ServerName, "minVersion": "1.3"}
	if server {
		value["certificates"] = []model.Object{{"certificate": strings.Split(strings.TrimSpace(secret.Certificate), "\n"), "key": strings.Split(strings.TrimSpace(secret.CertificateKey), "\n")}}
	} else if secret.TrustCertificate != "" {
		value["disableSystemRoot"] = true
		value["certificates"] = []model.Object{{"certificate": strings.Split(strings.TrimSpace(secret.TrustCertificate), "\n"), "usage": "verify"}}
	}
	if strings.HasPrefix(service.Protocol, "masque-") {
		value["alpn"] = []string{strings.TrimPrefix(service.Protocol, "masque-")}
		if server {
			value["alpn"] = []string{"h2", "h3"}
		}
	}
	return value
}
