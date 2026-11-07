package installer

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/serviceuser"

	"vpn-script/internal/platform/safeio"
)

const service = "vpn-nhop.service"
const account = serviceuser.Name
const marker = "# Managed by vpn-script N-hop v1"
const unitText = marker + `
[Unit]
Description=N-hop VPN node (Xray)
After=network-online.target nss-lookup.target
Wants=network-online.target

[Service]
Type=simple
User=vpn-nhop
Group=vpn-nhop
ExecStart=/opt/vpn-nhop/current/xray run -config /opt/vpn-nhop/current/config.json
Restart=on-failure
RestartSec=5
RestartPreventExitStatus=23
LimitNOFILE=65536
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
LockPersonality=true
UMask=0077

[Install]
WantedBy=multi-user.target
`

func (i *Installer) command(arguments ...string) ([]byte, error) {
	result, err := i.Run(i.Context, 30*time.Second, arguments...)
	if err == nil && result.Success {
		return result.Output, nil
	}
	path := filepath.Join(i.Paths.Base, "last-error.log")
	if writeErr := safeio.Atomic(path, result.Output, 0600); writeErr != nil {
		return nil, fmt.Errorf("%s failed; could not save protected diagnostic", filepath.Base(arguments[0]))
	}
	return nil, fmt.Errorf("%s failed; protected diagnostic: %s", filepath.Base(arguments[0]), path)
}

func (i *Installer) probe(arguments ...string) bool {
	result, err := i.Run(i.Context, 30*time.Second, arguments...)
	return err == nil && result.Success
}

func (i *Installer) systemctl(action string) error {
	_, err := i.command("systemctl", action, service)
	return err
}

func (i *Installer) reload() error {
	_, err := i.command("systemctl", "daemon-reload")
	return err
}

func (i *Installer) existingGroup() (int, error) {
	identity, err := serviceuser.Inspect(i.command)
	return identity.GID, err
}

func unitFor(data model.NodeConfig) string {
	if data.SchemaVersion != 2 {
		return unitText
	}
	unit := strings.Replace(unitText, "ExecStart=/opt/vpn-nhop/current/xray run -config /opt/vpn-nhop/current/config.json",
		"ExecStart=/opt/vpn-nhop/current/nhop-node --supervise /opt/vpn-nhop/current", 1)
	// Let the supervisor order child shutdown; systemd retains the final cgroup kill bound.
	unit = strings.Replace(unit, "[Service]\n", "[Service]\nKillMode=mixed\nTimeoutStopSec=5s\n", 1)
	names := make([]string, 0, len(data.Programs))
	for _, program := range data.Programs {
		names = append(names, program.Name)
		if program.Name == "sing-box" {
			// sing-box monitors interface/route changes through unprivileged netlink sockets.
			unit = strings.Replace(unit, "RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6\n",
				"RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK\n", 1)
		}
	}
	return strings.Replace(unit, "N-hop VPN node (Xray)", "N-hop VPN node ("+strings.Join(names, ", ")+")", 1)
}
