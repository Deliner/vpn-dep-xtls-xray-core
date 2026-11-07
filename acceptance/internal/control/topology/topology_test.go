package topology_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/control/topology"
)

func example(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../examples/nhop/topology.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestMixedTopology(t *testing.T) {
	top, err := topology.Load("../../../examples/nhop/topology.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(top.Order, ",") != "exit,relay,entry" || len(top.Services) != 6 {
		t.Fatalf("unexpected topology: %+v", top.Order)
	}
	if top.Successors["entry.reality"] != "relay.shadowsocks" || top.Successors["exit.reality"] != "" {
		t.Fatal("mixed-hop route not preserved")
	}
}

func TestInvalidYAMLAndPortContracts(t *testing.T) {
	raw := example(t)
	cases := map[string]string{
		"duplicate": "version: 1\nversion: 1\n", "alias": "a: &one {}\nb: *one\n",
		"bool-port":       strings.Replace(raw, "port: 443", "port: true", 1),
		"zero-port":       strings.Replace(raw, "port: 22222", "port: 0", 1),
		"large-port":      strings.Replace(raw, "port: 443", "port: 65536", 1),
		"conflict":        strings.Replace(raw, "port: 8443", "port: 443", 1),
		"unknown-field":   strings.Replace(raw, "server_name:", "sever_name:", 1),
		"wrong-ssh":       strings.Replace(raw, "~/.ssh-custom/", "~/.ssh/", 1),
		"missing-service": strings.Replace(raw, "entry.reality,", "entry.missing,", 1),
		"node-cycle":      strings.Replace(raw, "entry.shadowsocks, relay.reality", "relay.reality, entry.shadowsocks", 1),
		"closed-conflict": strings.Replace(raw, "closed_ports: [22]", "closed_ports: [22222]", 1),
		"client-port":     strings.Replace(raw, "socks_port: 10808", "socks_port: false", 1),
	}
	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "topology.yaml")
			if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := topology.Load(path); err == nil {
				t.Fatal("invalid topology accepted")
			}
		})
	}
}
