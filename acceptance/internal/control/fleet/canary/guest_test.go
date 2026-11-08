package canary

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/node/fleet/service"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func guestRoot(t *testing.T) string {
	t.Helper()
	root := os.Getenv("NHOP_CANARY_GUEST")
	if root == "" {
		t.Skip("native test is delivered by just fleet-canary-test")
	}
	host, err := os.Hostname()
	require(t, err)
	if runtime.GOOS != "linux" || os.Geteuid() != 0 || os.Getegid() != 0 || !strings.HasPrefix(root, "/var/lib/nhop-canary-run-") || filepath.Clean(root) != root {
		t.Fatal("guest test requires the private root staging directory in an owned Linux VM")
	}
	if host != "lima-fleet-a" && host != "lima-fleet-b" && host != "lima-fleet-c" {
		t.Fatal("refusing a machine outside the three-node Fleet lab")
	}
	require(t, safeio.CheckAccess(root, os.ModeDir|0700, 0, 0))
	return root
}

func TestFleetGuestGuard(t *testing.T) {
	root := guestRoot(t)
	for _, path := range []string{service.Directory, agentState, service.UnitPath, service.UnitPath + ".d", nodeBase,
		"/etc/systemd/system/vpn-nhop.service", "/etc/systemd/system/vpn-nhop.service.d", "/etc/ufw/applications.d/vpn-nhop",
		"/run/systemd/system/" + service.Name, "/run/systemd/system/" + service.Name + ".d",
		"/run/systemd/system/vpn-nhop.service", "/run/systemd/system/vpn-nhop.service.d",
		"/run/systemd/transient/" + controllerUnit, "/etc/systemd/system/" + controllerUnit} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("refusing an existing managed resource: %s", path)
		}
	}
	result, err := process.Run(t.Context(), time.Second, "id", "vpn-nhop")
	if err == nil || result.Success {
		t.Fatal("refusing an existing VPN account")
	}
	require(t, safeio.NewJSON(filepath.Join(root, "owns-node.json"), true))
}

func localCommand(t *testing.T, args ...string) []byte {
	t.Helper()
	result, err := process.Run(context.Background(), time.Minute, args...)
	if err != nil || !result.Success {
		t.Fatalf("guest command %q failed: %v: %s", args[0], err, result.Output)
	}
	return result.Output
}

func TestFleetGuestClean(t *testing.T) {
	root := guestRoot(t)
	var owned bool
	require(t, safeio.LoadJSON(filepath.Join(root, "owns-node.json"), &owned))
	if !owned {
		t.Fatal("missing preflight ownership receipt")
	}
	for _, unit := range []string{service.Name, "vpn-nhop.service"} {
		result, err := process.Run(context.Background(), time.Minute, "systemctl", "disable", "--now", "--", unit)
		if _, exists := os.Lstat("/etc/systemd/system/" + unit); exists == nil && (err != nil || !result.Success) {
			t.Fatal("cannot stop owned unit; retaining state", err)
		}
	}
	for _, path := range []string{service.UnitPath, service.Directory, agentState, "/etc/systemd/system/vpn-nhop.service", nodeBase} {
		require(t, os.RemoveAll(path))
	}
	localCommand(t, "systemctl", "daemon-reload")
	_, _ = process.Run(context.Background(), time.Minute, "systemctl", "reset-failed", service.Name, "vpn-nhop.service")
	if result, err := process.Run(context.Background(), time.Second, "id", "vpn-nhop"); err == nil && result.Success {
		localCommand(t, "userdel", "vpn-nhop")
	}
	require(t, os.Remove(filepath.Join(root, "owns-node.json")))
}
