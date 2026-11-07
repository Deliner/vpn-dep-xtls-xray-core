package protocol

func research() []Spec {
	groups := map[string][]string{
		"Uses the Shadowsocks data plane; separate Outline management API is not implemented":                {"outline"},
		"Legacy insecure protocol; no supported adapter":                                                     {"pptp"},
		"SSH account and tunnelling policy require a separate adapter; existing management SSH is preserved": {"ssh-tunnel"},
		"Separate runtime adapter and payload/rollback tests required":                                       {"mieru", "sudoku", "shadowquic", "trusttunnel", "restls", "cloak", "naiveproxy-standalone"},
		"L3 adapter, interface/address planning and Linux namespace tests required":                          {"wireguard", "amneziawg-3.1", "openvpn", "lightway", "tailcat", "ikev2-ipsec", "l2tp-ipsec", "openconnect"},
		"Authoritative DNS delegation, resolver and external path tests required":                            {"dnstt", "vaydns", "noizdns", "slipstream", "soar", "masterdns", "xdns"},
		"Transport modifier; separate config/compatibility tests required":                                   {"finalmask", "udphop", "xmc", "xicmp", "mimic", "aivpn", "mirage"},
		"External infrastructure and its credentials required":                                               {"tor", "obfs4", "snowflake", "xdrive", "gooserelay", "webtunnel", "psiphon", "conduit", "conjure", "webrtc", "hysteria-realm", "serverless-web-functions"},
		"Provider-specific service; not a generic self-hosted protocol":                                      {"mullvad-obfuscation", "ivpn-pq", "dausos", "nym", "obscura"},
		"Standard, library or engine feature; not an independently deployable listener":                      {"quic-v2", "ech", "dtls-1.3", "multipath-quic", "webtransport", "webtransport-fallback", "water", "openssh-pq", "mihomo-tun-dns", "masque-extensions", "kindling"},
		"Routing primitive; reverse roles and independent failure tests required":                            {"xray-reverse"},
	}
	var result []Spec
	for requirement, ids := range groups {
		for _, id := range ids {
			result = append(result, Spec{ID: id, Status: "research-only", Requirement: requirement})
		}
	}
	return result
}
