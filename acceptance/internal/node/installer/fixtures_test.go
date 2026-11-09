package installer

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

type backend struct {
	responses map[string]string
	fail      string
	active    bool
	enabled   bool
	ports     string
	calls     []string
}

func (b *backend) run(ctx context.Context, _ time.Duration, args ...string) (process.Result, error) {
	command := strings.Join(args, " ")
	b.calls = append(b.calls, command)
	if ctx.Err() != nil {
		return process.Result{}, ctx.Err()
	}
	if command == b.fail {
		b.fail = ""
		return process.Result{}, fmt.Errorf("injected failure")
	}
	result := process.Result{Success: true}
	if output, ok := b.response(command); ok {
		result.Output = []byte(output)
		return result, nil
	}
	switch command {
	case "systemctl is-active --quiet " + service:
		result.Success = b.active
	case "systemctl is-enabled --quiet " + service:
		result.Success = b.enabled
	case "systemctl show -p MainPID --value " + service:
		result.Output = []byte("123\n")
	case "ss -H -O -lnptue":
		result.Output = []byte(b.ports)
	case "systemctl stop " + service:
		b.active = false
	case "systemctl restart " + service, "systemctl start " + service:
		b.active = true
	case "systemctl enable " + service, "systemctl disable " + service:
		b.enabled = strings.HasPrefix(command, "systemctl enable ")
	}
	return result, nil
}

func (b *backend) response(command string) (string, bool) {
	if output, ok := b.responses[command]; ok {
		return output, true
	}
	switch command {
	case "systemctl show -p LoadState -p ActiveState -p UnitFileState " + service:
		active, enabled := "inactive", "disabled"
		if b.active {
			active = "active"
		}
		if b.enabled {
			enabled = "enabled"
		}
		return fmt.Sprintf("LoadState=loaded\nActiveState=%s\nUnitFileState=%s\n", active, enabled), true
	case "getent passwd " + account, "getent passwd 987":
		return fmt.Sprintf("%s:x:987:%d::/nonexistent:/usr/sbin/nologin\n", account, testGroup()), true
	case "getent group " + account, fmt.Sprintf("getent group %d", testGroup()):
		return fmt.Sprintf("%s:x:%d:\n", account, testGroup()), true
	case "id -G " + account:
		return fmt.Sprint(testGroup()), true
	case "id -u " + account:
		return "987\n", true
	}
	return "", false
}

func testGroup() int {
	return max(1, os.Getegid())
}

func testGroupOwnership(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		require(t, os.Chown(path, -1, testGroup()))
	}
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T, previous bool) (*Installer, *backend, model.NodeConfig, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require(t, err)
	require(t, os.Chmod(root, 0750))
	testGroupOwnership(t, root)
	i := New(context.Background())
	i.Paths = Paths{Base: root, Unit: filepath.Join(root, "unit"), Firewall: filepath.Join(root, "ufw")}
	b := &backend{active: previous, enabled: previous, ports: "tcp LISTEN 0 4096 0.0.0.0:24443 0.0.0.0:* users:((\"xray\",pid=123,fd=7))\n"}
	i.Run, i.Sleep = b.run, func(time.Duration) {}
	data := model.NodeConfig{NodeID: "entry", TopologyID: "test", Generation: strings.Repeat("b", 64), Firewall: "preserve",
		Listeners: []model.Listener{{Address: "0.0.0.0", Port: 24443, Network: "tcp"}}}
	new := testRelease(t, root, data.Generation)
	if previous {
		old := testRelease(t, root, strings.Repeat("a", 64))
		require(t, os.Symlink(old, filepath.Join(root, "current")))
		require(t, os.WriteFile(i.Paths.Unit, []byte(marker+"\nold unit\n"), 0644))
		require(t, os.WriteFile(i.Paths.Firewall, []byte(marker+"\nold ports\n"), 0644))
		require(t, safeio.JSON(filepath.Join(root, "state.json"), State{TopologyID: data.TopologyID, NodeID: data.NodeID,
			Generation: strings.Repeat("a", 64), ConfigSHA256: safeio.Hash([]byte("{}")), BinarySHA256: safeio.Hash([]byte("binary")), Firewall: "preserve"}))
	}
	return i, b, data, new
}

func testRelease(t *testing.T, root, generation string) string {
	t.Helper()
	path := filepath.Join(root, "releases", generation)
	require(t, os.MkdirAll(path, 0750))
	require(t, os.Chmod(filepath.Dir(path), 0750))
	require(t, os.Chmod(path, 0750))
	require(t, safeio.Atomic(filepath.Join(path, "xray"), []byte("binary"), 0755))
	require(t, safeio.Atomic(filepath.Join(path, "config.json"), []byte("{}"), 0640))
	testGroupOwnership(t, root, filepath.Dir(path), path, filepath.Join(path, "config.json"))
	return path
}

func programRelease(t *testing.T, data *model.NodeConfig, directory string) {
	t.Helper()
	data.SchemaVersion = 2
	data.Programs = []model.Program{{Name: "xray"}, {Name: "sing-box"}}
	for _, name := range []string{"xray.json", "sing-box", "sing-box.json", "libcronet.so", "nhop-node", "processes.json"} {
		mode := os.FileMode(0755)
		if strings.HasSuffix(name, ".json") {
			mode = 0640
		}
		require(t, safeio.Atomic(filepath.Join(directory, name), []byte("fixture "+name), mode))
		if strings.HasSuffix(name, ".json") {
			testGroupOwnership(t, filepath.Join(directory, name))
		}
	}
	require(t, os.Remove(filepath.Join(directory, "config.json")))
	executable, err := os.Executable()
	require(t, err)
	raw, err := safeio.Read(executable, 100_000_000)
	require(t, err)
	require(t, os.WriteFile(filepath.Join(directory, "nhop-node"), raw, 0755))
}

func legacyProgram(t *testing.T) (*Installer, *backend, model.NodeConfig, string) {
	t.Helper()
	i, backend, data, directory := setup(t, false)
	programRelease(t, &data, directory)
	require(t, os.WriteFile(filepath.Join(directory, "nhop-node"), []byte("previous committed build"), 0755))
	require(t, i.activate(data, directory))
	var state State
	require(t, safeio.LoadJSON(filepath.Join(i.Paths.Base, "state.json"), &state))
	state.ReleaseID = ""
	require(t, safeio.JSON(filepath.Join(i.Paths.Base, "state.json"), state))
	return i, backend, data, directory
}

func candidateProgram(t *testing.T, i *Installer, data model.NodeConfig) string {
	t.Helper()
	stage, err := os.MkdirTemp(filepath.Join(i.Paths.Base, "releases"), ".candidate-")
	require(t, err)
	require(t, os.Chmod(stage, 0750))
	testRelease(t, i.Paths.Base, filepath.Base(stage))
	programRelease(t, &data, stage)
	return stage
}

func kernelListener(t *testing.T, network, address string) model.Listener {
	t.Helper()
	var close io.Closer
	var endpoint net.Addr
	if strings.HasPrefix(network, "tcp") {
		listener, err := net.Listen(network, net.JoinHostPort(address, "0"))
		require(t, err)
		close, endpoint = listener, listener.Addr()
	} else {
		listener, err := net.ListenPacket(network, net.JoinHostPort(address, "0"))
		require(t, err)
		close, endpoint = listener, listener.LocalAddr()
	}
	t.Cleanup(func() { require(t, close.Close()) })
	_, port, err := net.SplitHostPort(endpoint.String())
	require(t, err)
	var number int
	_, err = fmt.Sscan(port, &number)
	require(t, err)
	return model.Listener{Network: strings.TrimRight(network, "46"), Port: number, Address: address}
}
