// Package service defines the persistent Linux Fleet agent unit. VPN engine
// units remain owned by the node installer and outlive stopping this agent.
package service

import (
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"
)

const Name = "nhop-fleet-agent.service"
const Directory = "/var/lib/nhop-fleet-service"
const UnitPath = "/etc/systemd/system/" + Name
const AccessDeniedExit = 78

func Unit(executable, state string, interval time.Duration) ([]byte, error) {
	if interval < time.Second || interval > time.Hour || interval%time.Second != 0 {
		return nil, fmt.Errorf("service poll interval must be a whole number of seconds in 1..3600")
	}
	for _, value := range []string{executable, state} {
		if err := PermanentPath(value); err != nil {
			return nil, err
		}
	}
	arguments := []string{executable, "agent-run", "--state", state, "--interval", interval.String()}
	for index, value := range arguments {
		arguments[index] = quote(value)
	}
	return []byte(preamble + "ExecStart=:" + strings.Join(arguments, " ") + "\n" + supervision), nil
}

func PermanentPath(value string) error {
	if !path.IsAbs(value) || path.Clean(value) != value || value == "/" || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("service requires a clean absolute path without control characters")
	}
	for _, root := range []string{"/home", "/root", "/tmp", "/var/tmp", "/run"} {
		if value == root || strings.HasPrefix(value, root+"/") {
			return fmt.Errorf("service path must be permanent and visible under ProtectHome/PrivateTmp")
		}
	}
	return nil
}

func quote(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%").Replace(value) + `"`
}

const preamble = `# Managed N-hop Fleet agent; stopping it leaves the VPN service running.
[Unit]
Description=N-hop Fleet pull agent
Wants=network-online.target
After=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=10

[Service]
Type=exec
User=root
Group=root
`

const supervision = `Restart=on-failure
RestartPreventExitStatus=78
RestartSec=5s
KillMode=mixed
TimeoutStopSec=120s
UMask=0077
LimitNOFILE=65536
NoNewPrivileges=yes
ProtectHome=yes
PrivateTmp=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
LockPersonality=yes
# No RuntimeDirectory or ExecStop: durable credentials and VPN state are retained.

[Install]
WantedBy=multi-user.target
`
