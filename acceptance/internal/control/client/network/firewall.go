package network

import (
	"fmt"
	"net/netip"
	"strings"

	"vpn-script/internal/control/client/profile"
)

// Firewall creates only our table. There is no established-connection bypass:
// sockets opened before start must obey the same egress policy after activation.
func Firewall(imported profile.Imported, options Options) (string, error) {
	if err := Validate(imported, options); err != nil {
		return "", err
	}
	var body strings.Builder
	fmt.Fprintf(&body, "table inet %s {\n chain output {\n  type filter hook output priority 50; policy drop;\n", Table)
	fmt.Fprintln(&body, "  oifname \"lo\" accept")
	writeEntry(&body, imported, options)
	writeLinkControl(&body)
	if options.IPv6 == "block" {
		fmt.Fprintln(&body, "  meta nfproto ipv6 counter drop")
	}
	fmt.Fprintf(&body, "  oifname %q accept\n  counter drop\n }\n", options.Interface)
	fmt.Fprintln(&body, " chain forward {\n  type filter hook forward priority 50; policy drop;\n }")
	fmt.Fprintln(&body, "}")
	return body.String(), nil
}

// BootFirewall needs no profile, account lookup, route, DNS or TUN. It keeps
// application egress closed while the outer interface is being configured.
func BootFirewall() string {
	var body strings.Builder
	fmt.Fprintf(&body, "table inet %s {\n chain output {\n  type filter hook output priority 50; policy drop;\n", BootTable)
	fmt.Fprintln(&body, "  oifname \"lo\" accept")
	writeLinkControl(&body)
	fmt.Fprintln(&body, "  counter drop\n }\n chain forward {\n  type filter hook forward priority 50; policy drop;\n }\n}")
	return body.String()
}

func writeLinkControl(body *strings.Builder) {
	// IPv6 link discovery maintains the outer network, including IPv6 VPN entries.
	fmt.Fprintln(body, "  ip6 hoplimit 255 icmpv6 type { nd-router-solicit, nd-neighbor-solicit, nd-neighbor-advert } accept")
	fmt.Fprintln(body, "  meta skuid 0 ip daddr 255.255.255.255 udp sport 68 udp dport 67 accept")
	fmt.Fprintln(body, "  meta skuid 0 ip6 daddr ff02::1:2 udp sport 546 udp dport 547 accept")
}

func writeEntry(body *strings.Builder, imported profile.Imported, options Options) {
	e := imported.Metadata.EntryEndpoint
	family := "ip"
	if netip.MustParseAddr(e.Address).Is6() {
		family = "ip6"
	}
	for _, transport := range strings.Split(e.Network, ",") {
		fmt.Fprintf(body, "  meta skuid %d %s daddr %s %s dport %d accept\n", options.EngineUID, family, e.Address, transport, e.Port)
	}
}
