package app

import (
	"flag"
	"fmt"
	"io"

	"vpn-script/internal/control/client/network"
)

type options struct {
	bundle, route, profile, output string
	network                        network.Options
}

func parse(command string, args []string, output io.Writer) (options, error) {
	value := options{network: network.Defaults(0)}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(output)
	switch command {
	case "import":
		flags.StringVar(&value.bundle, "bundle", "", "trusted exported clients directory")
		flags.StringVar(&value.route, "route", "main", "exported route name")
		flags.StringVar(&value.output, "output", "", "new private profile file")
	case "plan":
		flags.StringVar(&value.profile, "profile", "", "imported profile file")
		flags.StringVar(&value.output, "output", "", "new private plan directory")
		flags.IntVar(&value.network.EngineUID, "engine-uid", 0, "dedicated non-root native engine UID")
		flags.StringVar(&value.network.Interface, "interface", "nhop0", "private TUN interface")
		flags.StringVar(&value.network.IPv6, "ipv6", "tunnel", "tunnel or block")
		flags.StringVar(&value.network.DNSIP, "dns-ip", "1.1.1.1", "HTTPS DNS server IP reached through the tunnel")
		flags.StringVar(&value.network.DNSName, "dns-name", "cloudflare-dns.com", "HTTPS DNS certificate name")
		flags.IntVar(&value.network.MTU, "mtu", 1280, "TUN MTU")
	default:
		return value, fmt.Errorf("unknown client command")
	}
	if err := flags.Parse(args); err != nil {
		return value, err
	}
	if flags.NArg() != 0 || value.output == "" || command == "import" && value.bundle == "" || command == "plan" && value.profile == "" {
		return value, fmt.Errorf("missing required paths or unexpected arguments")
	}
	return value, nil
}
