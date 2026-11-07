package protocol

func proxies() []Spec {
	return []Spec{
		{"vmess-tls", "xray", true, false, "tcp", true, "implemented", "Legacy compatibility profile; AEAD VMess"},
		{"vmess-ws", "xray", true, false, "tcp", true, "implemented", "Legacy compatibility profile; AEAD VMess over WS/TLS"},
		{"vless-encryption", "xray", false, false, "tcp", true, "implemented", "ML-KEM-768 authentication; native TCP; no TLS camouflage"},
		{"naive-h2", "sing-box", true, false, "tcp", true, "implemented", "Publicly trusted TLS recommended; Linux libcronet"},
		{"naive-h3", "sing-box", true, false, "udp", true, "implemented", "UDP reachable; publicly trusted TLS recommended; Linux libcronet"},
		{"vless-reality", "xray", false, true, "tcp", true, "implemented", "REALITY TLS donor"},
		{"shadowsocks-2022", "xray", false, false, "tcp,udp", true, "implemented", ""},
		{"shadowsocks", "xray", false, false, "tcp,udp", true, "implemented", "chacha20-ietf-poly1305"},
		{"vless-tls", "xray", true, false, "tcp", true, "implemented", ""},
		{"vless-ws", "xray", true, false, "tcp", true, "implemented", ""},
		{"vless-xhttp", "xray", true, false, "tcp", true, "implemented", "TLS; no direct REALITY+XHTTP"},
		{"trojan", "xray", true, false, "tcp", true, "implemented", ""},
		{"trojan-ws", "xray", true, false, "tcp", true, "implemented", ""},
		{"masque-h2", "xray", true, false, "tcp,udp", true, "implemented", "CONNECT-IP; IP destinations"},
		{"masque-h3", "xray", true, false, "tcp,udp", true, "implemented", "CONNECT-IP; IP destinations, UDP reachable"},
		{"anytls", "sing-box", true, false, "tcp", true, "implemented", ""},
		{"tuic", "sing-box", true, false, "udp", true, "implemented", "TUIC v5; UDP reachable"},
		{"hysteria2", "sing-box", true, false, "udp", true, "implemented", "UDP reachable"},
		{"hysteria2-salamander", "sing-box", true, false, "udp", true, "implemented", "UDP reachable"},
		{"hysteria2-gecko", "sing-box", true, false, "udp", true, "implemented", "UDP reachable; matching Gecko implementation"},
		{"snell-v5", "sing-box", false, false, "tcp", true, "implemented", "v4-compatible wire; no Snell QUIC mode"},
		{"snell-v6", "sing-box", false, false, "tcp", true, "implemented", ""},
		{"shadowtls-v3", "sing-box", false, true, "tcp", true, "implemented", "ShadowTLS v3 + inner Shadowsocks-2022 + UoT"},
	}
}
