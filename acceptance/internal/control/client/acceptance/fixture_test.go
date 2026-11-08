package acceptance

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

func fixture(t *testing.T) session.Request {
	t.Helper()
	directory := t.TempDir()
	for _, path := range []string{filepath.Dir(directory), directory} {
		if err := os.Chmod(path, 0711); err != nil {
			t.Fatal(err)
		}
	}
	request := session.Request{Directory: filepath.Join(directory, "active"), Network: network.Defaults(20000), EngineGID: 20000}
	request.Native = artifact(t, "xray")
	request.Frontend = artifact(t, "sing-box")
	request.Cronet = artifact(t, "libcronet.so")
	password := "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY="
	config := xray.Client(model.Object{"protocol": "shadowsocks", "tag": "proxy", "settings": model.Object{"servers": []model.Object{{
		"address": "198.51.100.7", "port": 24443, "method": model.SSMethod, "password": password}}}}, 12080)
	path := filepath.Join(directory, "input.json")
	if err := safeio.JSON(path, config); err != nil {
		t.Fatal(err)
	}
	value := profile.Imported{SchemaVersion: 1, Topology: "fixture", Route: "main", Metadata: model.Profile{Engine: "xray", RuntimeVersion: xray.Version,
		Path: []string{"entry.proxy"}, HopCount: 1, EntryNode: "entry", ExitNode: "entry", SOCKSPort: 12080,
		EntryEndpoint: model.ClientEndpoint{Address: "198.51.100.7", Port: 24443, Network: "tcp,udp"}}}
	if err := safeio.LoadJSON(path, &value.Config); err != nil {
		t.Fatal(err)
	}
	value.Metadata.ConfigSHA256, _ = safeio.Digest(value.Config)
	request.Profile = value
	startExit(t, directory, password, request.Native.Path)
	return request
}

func artifact(t *testing.T, name string) session.Artifact {
	t.Helper()
	path := filepath.Join(os.Getenv("NHOP_CLIENT_RUNTIME"), name)
	digest, err := safeio.FileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	return session.Artifact{Path: path, SHA256: digest}
}

func startExit(t *testing.T, directory, password, binary string) {
	t.Helper()
	server := model.Object{"log": model.Object{"loglevel": "warning"}, "inbounds": []model.Object{{"listen": "198.51.100.7", "port": 24443,
		"protocol": "shadowsocks", "settings": model.Object{"method": model.SSMethod, "password": password, "network": "tcp,udp"}}},
		"outbounds": []model.Object{{"protocol": "freedom", "settings": model.Object{"finalRules": []model.Object{{
			"action": "allow", "network": "tcp,udp", "ip": []string{"198.18.0.2/32", "2001:db8:feed::2/128"}, "port": "443,18080,18081"}}}}}}
	path := filepath.Join(directory, "exit.json")
	if err := safeio.JSON(path, server); err != nil {
		t.Fatal(err)
	}
	command := process.Command(context.Background(), binary, "run", "-config", path)
	command.Stdout, command.Stderr = os.Stderr, os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { process.Stop(command); command.Wait() })
}

func origin(t *testing.T, address string) {
	t.Helper()
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go func() { defer connection.Close(); io.Copy(connection, connection) }()
		}
	}()
}

func checkIsolation(t *testing.T) {
	t.Helper()
	hostname, err := os.Hostname()
	ci := os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("GITHUB_REPOSITORY") == "Deliner/vpn-dep-xtls-xray-core" && os.Getenv("NHOP_CLIENT_CI") == "linux/"+runtime.GOARCH
	if err != nil || hostname != "lima-payload" && !ci || runtime.GOOS != "linux" || os.Geteuid() != 0 {
		t.Fatal("session acceptance requires an owned root Linux guest or runner")
	}
	for _, namespace := range []string{"net", "mnt"} {
		self, err := os.Readlink("/proc/self/ns/" + namespace)
		initial, initialErr := os.Readlink("/proc/1/ns/" + namespace)
		if err != nil || initialErr != nil || self == initial {
			t.Fatal("session acceptance requires private namespaces")
		}
	}
	t.Logf("native client acceptance: %s/%s; private mount and network namespaces", runtime.GOOS, runtime.GOARCH)
}

type controller struct {
	command *exec.Cmd
	encoder *json.Encoder
	decoder *json.Decoder
}

func newController(t *testing.T, request session.Request) (*controller, response) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := process.Command(context.Background(), "ip", "netns", "exec", "nhop-client-inner", executable, "-test.run=^TestSessionHelper$")
	command.Env = append(os.Environ(), "NHOP_SESSION_HELPER=1")
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	value := &controller{command: command, encoder: json.NewEncoder(input), decoder: json.NewDecoder(output)}
	t.Cleanup(func() {
		value.call(t, "stop")
		input.Close()
		process.Stop(command)
		command.Wait()
		sessionLogs(t, request.Directory)
	})
	if err := value.encoder.Encode(request); err != nil {
		t.Fatal(err)
	}
	var initial response
	if err := value.decoder.Decode(&initial); err != nil {
		t.Fatal(err)
	}
	return value, initial
}

func (c *controller) call(t *testing.T, operation string) response {
	t.Helper()
	if err := c.encoder.Encode(operation); err != nil {
		t.Fatal(err)
	}
	var value response
	if err := c.decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func sessionLogs(t *testing.T, directory string) {
	t.Helper()
	for _, name := range []string{"native.log", "frontend.log", "status.json"} {
		if data, err := os.ReadFile(filepath.Join(directory, name)); err == nil {
			t.Logf("%s: %s", name, data)
		}
	}
}
