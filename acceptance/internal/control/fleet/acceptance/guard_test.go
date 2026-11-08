package acceptance

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

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func guard(t *testing.T) (string, string) {
	t.Helper()
	if os.Getenv("NHOP_FLEET_ACCEPTANCE") != "private-lima" {
		t.Skip("real Fleet installation is restricted to just fleet-agent-test in the owned Linux VM")
	}
	host, err := os.Hostname()
	require(t, err)
	if runtime.GOOS != "linux" || os.Geteuid() != 0 || os.Getegid() != 0 || host != "lima-payload" {
		t.Fatal("Fleet acceptance requires root in the owned Linux VM")
	}
	root, binary := os.Getenv("NHOP_FLEET_TEST_ROOT"), os.Getenv("NHOP_FLEET_BINARY")
	if !strings.HasPrefix(root, "/var/lib/nhop-fleet-acceptance-") || filepath.Clean(root) != root || binary != filepath.Join(root, "nhop-fleet") || os.Getenv("NHOP_FLEET_REPORT") != filepath.Join(root, "reports/fleet.json") {
		t.Fatal("untrusted Fleet acceptance paths")
	}
	require(t, safeio.CheckAccess(root, os.ModeDir|0700, 0, 0))
	for _, path := range []string{service.Directory, agentState, service.UnitPath, service.UnitPath + ".d", "/run/systemd/system/" + service.Name,
		"/run/systemd/system/" + service.Name + ".d", nodeBase, nodeUnit, nodeUnit + ".d", "/run/systemd/system/vpn-nhop.service", "/run/systemd/system/vpn-nhop.service.d", "/etc/ufw/applications.d/vpn-nhop"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("refusing existing managed resource: %s", path)
		}
	}
	result, err := process.Run(context.Background(), time.Second, "id", "vpn-nhop")
	if err == nil || result.Success {
		t.Fatal("refusing an existing VPN service account")
	}
	return root, binary
}

func command(t *testing.T, args ...string) []byte {
	t.Helper()
	result, err := process.Run(context.Background(), 3*time.Minute, args...)
	if err != nil || !result.Success {
		t.Fatalf("command %q failed: %v: %s", args[0], err, result.Output)
	}
	return result.Output
}

func (f *fixture) cli(t *testing.T, args ...string) []byte {
	t.Helper()
	return command(t, append([]string{f.binary}, args...)...)
}

func property(t *testing.T, unit, name string) string {
	t.Helper()
	return strings.TrimSpace(string(command(t, "systemctl", "show", "--property="+name, "--value", "--", unit)))
}

func eventually(t *testing.T, description string, limit time.Duration, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatal("timed out waiting for " + description)
}

func (f *fixture) cleanup(t *testing.T) {
	t.Helper()
	for _, pair := range [][2]string{{service.Name, service.UnitPath}, {"vpn-nhop.service", nodeUnit}} {
		_, exists := os.Lstat(pair[1])
		result, err := process.Run(context.Background(), 3*time.Minute, "systemctl", "disable", "--now", "--", pair[0])
		if exists == nil && (err != nil || !result.Success) {
			t.Errorf("cannot stop owned fixture service; retaining state: %v", err)
			return
		}
	}
	for _, path := range []string{service.UnitPath, service.Directory, agentState, nodeUnit, nodeBase} {
		if err := os.RemoveAll(path); err != nil {
			t.Error(err)
			return
		}
	}
	command(t, "systemctl", "daemon-reload")
	_, _ = process.Run(context.Background(), 30*time.Second, "systemctl", "reset-failed", service.Name, "vpn-nhop.service")
	if result, err := process.Run(context.Background(), time.Second, "id", "vpn-nhop"); err == nil && result.Success {
		command(t, "userdel", "vpn-nhop")
	}
}
