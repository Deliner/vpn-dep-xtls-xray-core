package systemd

import (
	"fmt"
	"strings"
	"unicode"
)

const BootService = "nhop-client-boot.service"

// BootUnit has no Condition/ExecCondition or stop cleanup: failure must fail
// Requires+After consumers, and stopping a unit must retain the kernel policy.
func BootUnit(executable string) ([]byte, error) {
	if err := servicePath(executable); err != nil {
		return nil, err
	}
	if strings.IndexFunc(executable, unicode.IsControl) >= 0 {
		return nil, fmt.Errorf("boot executable cannot contain control characters")
	}
	return []byte(bootPreamble + "ExecStart=:" + quote(executable) + " \"boot-protect\"\n" + bootConfinement), nil
}

const BootRequirement = "[Unit]\nRequires=" + BootService + "\nAfter=" + BootService + "\n"

const bootPreamble = `[Unit]
Description=N-hop early offline egress protection
DefaultDependencies=no
Wants=network-pre.target
Before=network-pre.target shutdown.target
After=local-fs.target systemd-modules-load.service systemd-sysctl.service
Conflicts=shutdown.target
RequiresMountsFor=/var/lib/nhop-client /usr

[Service]
Type=oneshot
RemainAfterExit=yes
TimeoutStartSec=30s
UMask=0077
`

const bootConfinement = `NoNewPrivileges=yes
ProtectHome=yes
PrivateTmp=yes
ProtectSystem=full
ProtectKernelTunables=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6 AF_NETLINK
LockPersonality=yes
# No ExecStop or RuntimeDirectory: protection and its journal survive unit stop.
`
