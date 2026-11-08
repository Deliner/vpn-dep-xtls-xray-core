package network

import (
	"fmt"
	"net/netip"
	"regexp"

	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/core/validate"
)

const Table = "nhop_client"
const BootTable = "nhop_client_boot"
const IPv4 = "172.30.255.1/30"
const IPv6 = "fdfe:dcba:9876::1/126"
const DNS = "172.30.255.2"
const RouteIndex = 30910
const AssistTable = RouteIndex + 1

type Options struct {
	Interface string `json:"interface"`
	EngineUID int    `json:"engine_uid"`
	IPv6      string `json:"ipv6"`
	DNSIP     string `json:"dns_ip"`
	DNSName   string `json:"dns_name"`
	MTU       int    `json:"mtu"`
}

func Defaults(uid int) Options {
	return Options{Interface: "nhop0", EngineUID: uid, IPv6: "tunnel", DNSIP: "1.1.1.1", DNSName: "cloudflare-dns.com", MTU: 1280}
}

func Validate(imported profile.Imported, options Options) error {
	if err := profile.Validate(imported); err != nil {
		return err
	}
	if options.EngineUID < 100 || int64(options.EngineUID) > 4294967294 {
		return fmt.Errorf("invalid service UID")
	}
	return ValidateSettings(options)
}

// ValidateSettings checks user policy before a service account is resolved.
func ValidateSettings(options Options) error {
	if !regexp.MustCompile(`^nhop[a-z0-9_-]{0,10}$`).MatchString(options.Interface) {
		return fmt.Errorf("invalid private TUN interface")
	}
	if options.IPv6 != "tunnel" && options.IPv6 != "block" || options.MTU < 1280 || options.MTU > 9000 {
		return fmt.Errorf("expected IPv6 tunnel/block policy and MTU 1280..9000")
	}
	ip, err := netip.ParseAddr(options.DNSIP)
	if err != nil || ip.Zone() != "" || ip.IsUnspecified() || ip.IsMulticast() || ip.Is4In6() || validate.Host(options.DNSName) != nil {
		return fmt.Errorf("invalid tunneled HTTPS DNS endpoint")
	}
	return nil
}
