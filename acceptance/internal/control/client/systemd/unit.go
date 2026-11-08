// Package systemd renders client and early protection units; it never installs them.
package systemd

import (
	"fmt"
	"path"
	"strconv"
	"strings"
	"unicode"

	"vpn-script/internal/control/client/network"
)

type Options struct {
	Executable string
	Profile    string
	Runtimes   string
	Network    network.Options
}

func Unit(options Options) ([]byte, error) {
	if err := network.ValidateSettings(options.Network); err != nil {
		return nil, err
	}
	for _, value := range []string{options.Executable, options.Profile, options.Runtimes} {
		if err := servicePath(value); err != nil {
			return nil, err
		}
	}
	policy := options.Network
	arguments := []string{options.Executable, "service-run", "--profile", options.Profile, "--runtimes", options.Runtimes,
		"--interface", policy.Interface, "--ipv6", policy.IPv6, "--dns-ip", policy.DNSIP, "--dns-name", policy.DNSName, "--mtu", strconv.Itoa(policy.MTU)}
	for i, argument := range arguments {
		if strings.IndexFunc(argument, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("service arguments cannot contain control characters")
		}
		arguments[i] = quote(argument)
	}
	prepare := "ExecStartPre=:" + quote(options.Executable) + " \"service-prepare\"\n"
	return []byte(preamble + prepare + "ExecStart=:" + strings.Join(arguments, " ") + "\n" + supervision), nil
}

func servicePath(value string) error {
	if !path.IsAbs(value) || path.Clean(value) != value || value == "/" {
		return fmt.Errorf("service requires clean absolute Linux file paths")
	}
	for _, root := range []string{"/home", "/root", "/tmp", "/var/tmp", "/run/nhop-client"} {
		if value == root || strings.HasPrefix(value, root+"/") {
			return fmt.Errorf("service path is hidden, temporary or reserved: %s", value)
		}
	}
	return nil
}

// ExecStart's ':' prefix disables environment expansion. Escape specifiers and
// quotes/backslashes, preserving literal dollars in executable and arguments.
func quote(value string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%").Replace(value)
	return `"` + escaped + `"`
}

const preamble = `# Manual start by default; service-enable registers early protection separately.
[Unit]
Description=N-hop Linux client with retained kill switch
Wants=network-online.target
Requires=systemd-resolved.service
After=network-online.target systemd-resolved.service
StartLimitIntervalSec=60
StartLimitBurst=10

[Service]
Type=exec
`

const supervision = `Restart=on-failure
RestartSec=2s
KillMode=mixed
TimeoutStopSec=20s
UMask=0077
LimitNOFILE=65536
NoNewPrivileges=yes
ProtectHome=yes
PrivateTmp=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
LockPersonality=yes
# The preflight validates/creates state before ExecStart enters a fresh namespace.
ExecPaths=-/run/nhop-client
# No ExecStop: stopping the service retains protection. Use nhop-client stop to disconnect.
# No RuntimeDirectory: controller state and firewall ownership must survive service restarts.
`
