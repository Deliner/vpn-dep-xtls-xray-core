package xray

import (
	"strings"
	"vpn-script/internal/core/model"
)

func userProtocol(service model.Service) string {
	if strings.HasPrefix(service.Protocol, "vmess-") {
		return "vmess"
	}
	return "vless"
}

func userInbound(service model.Service, secret model.Secret) model.Object {
	client := model.Object{"id": secret.UUID}
	if userProtocol(service) == "vmess" {
		return model.Object{"clients": []model.Object{client}}
	}
	if service.Protocol == model.Reality {
		client["flow"] = "xtls-rprx-vision"
	}
	decryption := "none"
	if service.Protocol == "vless-encryption" {
		decryption = "mlkem768x25519plus.native.600s." + secret.EncryptionSeed
	}
	return model.Object{"decryption": decryption, "clients": []model.Object{client}}
}

func userOutbound(service model.Service, secret model.Secret) model.Object {
	if userProtocol(service) == "vmess" {
		return model.Object{"id": secret.UUID, "security": "auto"}
	}
	encryption := "none"
	if service.Protocol == "vless-encryption" {
		encryption = "mlkem768x25519plus.native.1rtt." + secret.EncryptionKey
	}
	user := model.Object{"id": secret.UUID, "encryption": encryption}
	if service.Protocol == model.Reality {
		user["flow"] = "xtls-rprx-vision"
	}
	return user
}
