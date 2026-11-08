package canary

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/control/integration/traffic"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func hostFixture(t *testing.T) *fixture {
	t.Helper()
	root, repo := os.Getenv("NHOP_FLEET_LAB_ROOT"), os.Getenv("NHOP_FLEET_LAB_REPOSITORY")
	if root == "" {
		t.Skip("use just fleet-canary-test with three owned Linux VMs")
	}
	home, err := os.UserHomeDir()
	require(t, err)
	if runtime.GOOS != "darwin" || !filepath.IsAbs(repo) || filepath.Dir(root) != filepath.Join(repo, "build/fleet-lab/runs") || !strings.HasPrefix(filepath.Base(root), "run-") {
		t.Fatal("canary acceptance requires the owned macOS Lima lab")
	}
	require(t, safeio.CheckAccess(root, os.ModeDir|0700, os.Geteuid(), -1))
	limaHome := os.Getenv("LIMA_HOME")
	if filepath.Dir(limaHome) != filepath.Join(home, ".ssh-custom") || !strings.HasPrefix(filepath.Base(limaHome), "nhop-fleetlab-") {
		t.Fatal("Fleet SSH metadata must be in the dedicated ~/.ssh-custom directory")
	}
	owner, err := safeio.Read(filepath.Join(limaHome, ".nhop-owner"), 4096)
	require(t, err)
	if strings.TrimSpace(string(owner)) != repo {
		t.Fatal("Fleet lab belongs to another checkout")
	}
	return &fixture{root: root, repository: repo, lima: filepath.Join(repo, "build/lab/tools/bin/limactl"),
		remote: "/var/lib/nhop-canary-" + filepath.Base(root), addresses: map[string]string{},
		report: evidence{Machines: map[string]machine{}, Rollouts: map[string]wire.Rollout{}, Traffic: map[string]traffic.Report{}, Revisions: map[string][]wire.Node{}}}
}

func (f *fixture) command(t *testing.T, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := process.Command(ctx, f.lima, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		path := filepath.Join(f.root, fmt.Sprintf("command-failure-%d.log", time.Now().UnixNano()))
		saveErr := safeio.Atomic(path, append(raw, []byte(stderr.String())...), 0600)
		t.Fatalf("owned VM command failed: %v: %s; private output %s (write error: %v)", err, stderr.String(), path, saveErr)
	}
	return raw
}

func (f *fixture) guest(t *testing.T, node string, args ...string) []byte {
	t.Helper()
	return f.command(t, append([]string{"shell", "--workdir", "/home/nhop", node, "sudo", "--"}, args...)...)
}

func (f *fixture) put(t *testing.T, node, local, name string, mode string) {
	t.Helper()
	stage := "/home/nhop/" + filepath.Base(f.remote)
	f.command(t, "shell", "--workdir", "/home/nhop", node, "mkdir", "-p", "--", stage)
	f.command(t, "copy", local, node+":"+stage+"/"+name)
	f.guest(t, node, "install", "-m", mode, "--", stage+"/"+name, f.remote+"/"+name)
	f.command(t, "shell", "--workdir", "/home/nhop", node, "rm", "--", stage+"/"+name)
}

func (f *fixture) putJSON(t *testing.T, node, name string, value any) {
	t.Helper()
	path := filepath.Join(f.root, node+"-"+name)
	require(t, safeio.NewJSON(path, value))
	f.put(t, node, path, name, "0600")
}

func (f *fixture) guestTest(t *testing.T, node, name string, args ...string) []byte {
	t.Helper()
	values := []string{"env", "NHOP_CANARY_GUEST=" + f.remote}
	values = append(values, args...)
	values = append(values, f.remote+"/canary.test", "-test.run=^"+name+"$", "-test.v", "-test.timeout=3m")
	return f.guest(t, node, values...)
}

func decode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var value T
	require(t, json.Unmarshal(raw, &value))
	return value
}

func (f *fixture) property(t *testing.T, node, unit, key string) string {
	t.Helper()
	return strings.TrimSpace(string(f.guest(t, node, "systemctl", "show", "--property="+key, "--value", "--", unit)))
}

func (f *fixture) saveRemote(t *testing.T, node, remote, name string) []byte {
	t.Helper()
	raw := f.guest(t, node, "cat", "--", remote)
	require(t, safeio.Atomic(filepath.Join(f.root, fmt.Sprintf("%s-%s", node, name)), raw, 0600))
	return raw
}
