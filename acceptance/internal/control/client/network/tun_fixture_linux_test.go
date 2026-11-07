package network

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

func liveConfigs(t *testing.T, directory, trust string, options Options) profile.Imported {
	t.Helper()
	value := fixture(t)
	password := "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXoxMjM0NTY="
	server := model.Object{"log": model.Object{"loglevel": "warning"}, "inbounds": []model.Object{{"listen": "198.51.100.7", "port": 24443,
		"protocol": "shadowsocks", "settings": model.Object{"method": model.SSMethod, "password": password, "network": "tcp,udp"}}},
		"outbounds": []model.Object{{"protocol": "freedom", "settings": model.Object{"finalRules": []model.Object{{
			"action": "allow", "network": "tcp,udp", "ip": []string{"198.18.0.2/32", "2001:db8:feed::2/128"}, "port": "443,18080,18081"}}}}}}
	client := xray.Client(model.Object{"protocol": "shadowsocks", "tag": "proxy", "settings": model.Object{"servers": []model.Object{{
		"address": "198.51.100.7", "port": 24443, "method": model.SSMethod, "password": password}}}}, 12080)
	writeLiveJSON(t, directory, "server.json", server)
	writeLiveJSON(t, directory, "native.json", client)
	if err := safeio.LoadJSON(filepath.Join(directory, "native.json"), &value.Config); err != nil {
		t.Fatal(err)
	}
	value.Metadata.ConfigSHA256, _ = safeio.Digest(value.Config)
	options.DNSIP, options.DNSName = "198.18.0.2", "dns.example.invalid"
	tun, err := TUN(value, options)
	if err != nil {
		t.Fatal(err)
	}
	dns := tun["dns"].(model.Object)["servers"].([]model.Object)[0]
	dns["tls"].(model.Object)["certificate"] = []string{trust}
	writeLiveJSON(t, directory, "tun.json", tun)
	return value
}

func writeLiveJSON(t *testing.T, directory, name string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	// These credentials are synthetic and the unprivileged engine needs read access.
	if err := os.WriteFile(filepath.Join(directory, name), raw, 0644); err != nil {
		t.Fatal(err)
	}
}

func liveDirectory(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := os.Chmod(directory, 0755); err != nil {
		t.Fatal(err)
	}
	// TempDir's parent is private to this test too; only traversal is required.
	if err := os.Chmod(filepath.Dir(directory), 0711); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"xray", "sing-box", "libcronet.so"} {
		raw, err := os.ReadFile(filepath.Join(os.Getenv("NHOP_CLIENT_RUNTIME"), name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, name), raw, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}
