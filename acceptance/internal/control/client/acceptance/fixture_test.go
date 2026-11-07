package acceptance

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
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
