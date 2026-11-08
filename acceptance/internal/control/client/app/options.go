package app

import (
	"flag"
	"fmt"
	"io"

	"vpn-script/internal/control/client/network"
)

type options struct {
	bundle, route, profile, output, runtimes, binary string
	networkManager                                   string
	network                                          network.Options
}

func parse(command string, args []string, output io.Writer) (options, error) {
	value := options{network: network.Defaults(0)}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(output)
	if err := value.flags(command, flags); err != nil {
		return value, err
	}
	if err := flags.Parse(args); err != nil {
		return value, err
	}
	if flags.NArg() != 0 || !value.complete(command) {
		return value, fmt.Errorf("missing required paths or unexpected arguments")
	}
	return value, nil
}

func (value *options) flags(command string, flags *flag.FlagSet) error {
	switch command {
	case "import":
		flags.StringVar(&value.bundle, "bundle", "", "trusted exported clients directory")
		flags.StringVar(&value.route, "route", "main", "exported route name")
		flags.StringVar(&value.output, "output", "", "new private profile file")
	case "plan", "start", "service-run", "service-plan", "service-install":
		flags.StringVar(&value.profile, "profile", "", "trusted imported profile file")
		value.networkFlags(flags)
		if command == "plan" {
			flags.StringVar(&value.output, "output", "", "new private plan directory")
			flags.IntVar(&value.network.EngineUID, "engine-uid", 0, "dedicated non-root native engine UID")
		} else {
			flags.StringVar(&value.runtimes, "runtimes", "", "trusted runtime paths and SHA-256 inventory")
		}
		if command == "service-plan" {
			flags.StringVar(&value.binary, "binary", "", "absolute installed Linux client executable path")
			flags.StringVar(&value.output, "output", "", "new service unit file; does not install or activate")
		}
	case "service-enable":
		flags.StringVar(&value.networkManager, "network-manager", "", "active systemd-networkd.service or NetworkManager.service; auto-detect if empty")
	case "service-disable", "diagnostics", "stop", "reconnect", "resume", "service-prepare", "service-remove", "boot-protect":
	default:
		return fmt.Errorf("unknown client command")
	}
	return nil
}

func (value *options) networkFlags(flags *flag.FlagSet) {
	flags.StringVar(&value.network.Interface, "interface", "nhop0", "private TUN interface")
	flags.StringVar(&value.network.IPv6, "ipv6", "tunnel", "tunnel or block")
	flags.StringVar(&value.network.DNSIP, "dns-ip", "1.1.1.1", "HTTPS DNS server IP reached through the tunnel")
	flags.StringVar(&value.network.DNSName, "dns-name", "cloudflare-dns.com", "HTTPS DNS certificate name")
	flags.IntVar(&value.network.MTU, "mtu", 1280, "TUN MTU")
}

func (value options) complete(command string) bool {
	switch command {
	case "import":
		return value.bundle != "" && value.output != ""
	case "plan":
		return value.profile != "" && value.output != ""
	case "start", "service-run", "service-install":
		return value.profile != "" && value.runtimes != ""
	case "service-plan":
		return value.profile != "" && value.runtimes != "" && value.binary != "" && value.output != ""
	default:
		return true
	}
}
