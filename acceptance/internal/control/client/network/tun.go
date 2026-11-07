package network

import (
	"net/netip"

	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/core/model"
)

func TUN(imported profile.Imported, options Options) (model.Object, error) {
	if err := Validate(imported, options); err != nil {
		return nil, err
	}
	addresses := []string{IPv4}
	strategy := "prefer_ipv4"
	if options.IPv6 == "tunnel" {
		addresses = append(addresses, IPv6)
	} else {
		strategy = "ipv4_only"
	}
	entry := netip.MustParseAddr(imported.Metadata.EntryEndpoint.Address)
	inbound := model.Object{"type": "tun", "tag": "tun", "interface_name": options.Interface, "address": addresses,
		"mtu": options.MTU, "auto_route": true, "strict_route": true, "stack": "gvisor", "dns_mode": "hijack",
		"dns_address": []string{DNS}, "iproute2_table_index": RouteIndex, "iproute2_rule_index": RouteIndex,
		"route_exclude_address": []string{netip.PrefixFrom(entry, entry.BitLen()).String()}}
	dns := model.Object{"type": "https", "tag": "tunnel-dns", "server": options.DNSIP, "server_port": 443,
		"path": "/dns-query", "detour": "proxy", "tls": model.Object{"enabled": true, "server_name": options.DNSName}}
	return model.Object{"log": model.Object{"level": "warn"}, "inbounds": []model.Object{inbound},
		"outbounds": []model.Object{{"type": "socks", "tag": "proxy", "server": "127.0.0.1", "server_port": imported.Metadata.SOCKSPort, "version": "5"}},
		"dns":       model.Object{"servers": []model.Object{dns}, "final": "tunnel-dns", "strategy": strategy},
		"route": model.Object{"auto_detect_interface": true, "final": "proxy", "default_domain_resolver": "tunnel-dns", "rules": []model.Object{
			{"port": 53, "action": "hijack-dns"}, {"network": "udp", "port": 443, "action": "reject"}}}}, nil
}
