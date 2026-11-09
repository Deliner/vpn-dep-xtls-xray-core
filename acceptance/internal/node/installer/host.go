package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/journal"
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

func (i *Installer) preflight() error {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return fmt.Errorf("installation requires Linux with systemd and root privileges")
	}
	if info, err := os.Stat("/run/systemd/system"); err != nil || !info.IsDir() {
		return fmt.Errorf("installation requires systemd (target: Ubuntu 24.04)")
	}
	for _, name := range []string{"systemctl", "ss", "useradd", "id", "getent"} {
		if _, err := exec.LookPath(name); err != nil {
			return fmt.Errorf("missing host prerequisite: %s", name)
		}
	}
	if err := i.managedPaths(); err != nil {
		return err
	}
	unit, err := optionalFile(i.Paths.Unit)
	if err != nil {
		return err
	}
	if unit != nil && !strings.HasPrefix(string(unit), marker) {
		return fmt.Errorf("refusing to replace an unmanaged unit")
	}
	return nil
}

func (i *Installer) managedPaths() error {
	if err := managedBase(i.Paths.Base); err != nil {
		return err
	}
	paths := map[string]os.FileMode{i.Paths.Unit: 0644, i.Paths.Firewall: 0644,
		filepath.Join(i.Paths.Base, journal.Name): 0600,
		filepath.Join(i.Paths.Base, "state.json"): 0600, filepath.Join(i.Paths.Base, "install.lock"): 0600,
		filepath.Join(i.Paths.Base, "last-error.log"): 0600, filepath.Join(i.Paths.Base, "cache"): os.ModeDir | 0700,
		filepath.Join(i.Paths.Base, "releases"): os.ModeDir | 0750}
	for path, mode := range paths {
		if err := safeio.CheckAccess(path, mode, os.Geteuid(), -1); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func managedBase(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(path, 0700); err != nil {
			return err
		}
		return safeio.CheckAccess(path, os.ModeDir|0700, os.Geteuid(), -1)
	}
	if err != nil {
		return err
	}
	if info.Mode() != os.ModeDir|0700 && info.Mode() != os.ModeDir|0750 {
		return fmt.Errorf("unsafe managed access: %s; expected private directory; inspect before retrying", path)
	}
	return safeio.CheckAccess(path, info.Mode(), os.Geteuid(), -1)
}

func Architecture() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "aarch64", nil
	default:
		return "", fmt.Errorf("only amd64 and arm64 are supported")
	}
}

func optionalFile(path string) ([]byte, error) {
	raw, err := safeio.Read(path, 10_000_000)
	if os.IsNotExist(err) {
		return nil, nil
	}
	return raw, err
}

func restoreFile(path string, contents []byte, mode os.FileMode) error {
	if contents == nil {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return safeio.SyncDir(filepath.Dir(path))
	}
	return safeio.Atomic(path, contents, mode)
}

func (i *Installer) serviceState() (bool, bool, error) {
	raw, err := i.command("systemctl", "show", "-p", "LoadState", "-p", "ActiveState", "-p", "UnitFileState", service)
	if err != nil {
		return false, false, err
	}
	fields := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if key != "LoadState" && key != "ActiveState" && key != "UnitFileState" {
			return false, false, fmt.Errorf("unexpected systemd snapshot property")
		}
		if _, duplicate := fields[key]; !ok || duplicate {
			return false, false, fmt.Errorf("invalid systemd state snapshot")
		}
		fields[key] = value
	}
	if len(fields) != 3 {
		return false, false, fmt.Errorf("incomplete systemd state snapshot")
	}
	return stableServiceState(fields)
}

func stableServiceState(fields map[string]string) (bool, bool, error) {
	if fields["LoadState"] == "not-found" && fields["ActiveState"] == "inactive" && fields["UnitFileState"] == "" {
		return false, false, nil
	}
	if fields["LoadState"] != "loaded" ||
		fields["ActiveState"] != "active" && fields["ActiveState"] != "inactive" && fields["ActiveState"] != "failed" ||
		fields["UnitFileState"] != "enabled" && fields["UnitFileState"] != "disabled" {
		return false, false, fmt.Errorf("VPN service is not in a stable managed state; inspect before retrying")
	}
	return fields["ActiveState"] == "active", fields["UnitFileState"] == "enabled", nil
}

func (i *Installer) disableIfPresent() error {
	unit, err := optionalFile(i.Paths.Unit)
	if err != nil || unit == nil {
		return err
	}
	return i.systemctl("disable")
}
