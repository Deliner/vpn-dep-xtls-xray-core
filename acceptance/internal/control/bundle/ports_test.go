package bundle_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/control/bundle"
	"vpn-script/internal/control/topology"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
)

func portTopology(t *testing.T, tcp, udp int) *model.Topology {
	t.Helper()
	definition := model.Definition{Version: 1, Name: "ports", Nodes: map[string]*model.Node{"node": {
		Address: "192.0.2.1", Firewall: "ufw", SSH: model.SSH{User: "root", IdentityFile: "~/.ssh-custom/key", KnownHostsFile: "~/.ssh-custom/known_hosts"},
		Services: map[string]*model.Service{
			"tcp": {Protocol: model.Reality, Port: tcp, ServerName: "donor.example.invalid"},
			"udp": {Protocol: "tuic", Port: udp, ServerName: "vpn.example.invalid"},
		},
	}}, Routes: map[string]*model.Route{
		"tcp": {Path: []string{"node.tcp"}, Client: model.Client{SOCKSPort: 10808}},
		"udp": {Path: []string{"node.udp"}, Client: model.Client{SOCKSPort: 10809}},
	}}
	top, err := topology.Build(definition)
	must(t, err)
	return top
}

func TestSharedAndSSHNumberedUDPPortsReachManifestFirewallAndClients(t *testing.T) {
	for _, ports := range [][2]int{{443, 443}, {24443, 24443}, {24443, 22}, {24443, 22222}} {
		t.Run(fmt.Sprint(ports), func(t *testing.T) {
			top := portTopology(t, ports[0], ports[1])
			output := filepath.Join(t.TempDir(), "bundle")
			must(t, bundle.Render(top, output, fakeInstallers(t)))
			_, err := bundle.Load(output)
			must(t, err)
			data, err := manifest.Load(filepath.Join(output, "nodes", "node.json"))
			must(t, err)
			checkUDPExitListeners(t, data, 2)
			for index, network := range []string{"tcp", "udp"} {
				if !strings.Contains(installer.UFWProfile(data), fmt.Sprintf("%d/%s", ports[index], network)) {
					t.Fatal("firewall omitted a required transport")
				}
				if actual := clientPort(t, output, network); actual != fmt.Sprint(ports[index]) {
					t.Fatalf("%s client uses port %s instead of %d", network, actual, ports[index])
				}
			}
			if ports[0] != ports[1] && strings.Contains(installer.UFWProfile(data), fmt.Sprintf("%d/tcp", ports[1])) {
				t.Fatal("UDP listener opened the corresponding TCP/SSH port")
			}
		})
	}
}

func TestSingBoxOnlyExitIncludesPacketPolicyRuntime(t *testing.T) {
	top := portTopology(t, 24443, 24443)
	delete(top.Nodes["node"].Services, "tcp")
	delete(top.Routes, "tcp")
	top, err := topology.Build(top.Definition)
	must(t, err)
	output := filepath.Join(t.TempDir(), "bundle")
	must(t, bundle.Render(top, output, fakeInstallers(t)))
	data, err := manifest.Load(filepath.Join(output, "nodes", "node.json"))
	must(t, err)
	if len(data.Programs) != 2 {
		t.Fatal("sing-box exit omitted its packet policy runtime")
	}
	checkUDPExitListeners(t, data, 1)
}

func checkUDPExitListeners(t *testing.T, data model.NodeConfig, public int) {
	t.Helper()
	private := 0
	for _, listener := range data.Listeners {
		if !listener.Internal {
			public--
			continue
		}
		private++
		if listener.Tag != "bridge-udp-exit" || listener.Address != "127.0.0.1" {
			t.Fatalf("unbounded packet policy listener: %+v", listener)
		}
		if strings.Contains(installer.UFWProfile(data), fmt.Sprintf("%d/", listener.Port)) {
			t.Fatal("packet policy listener is exposed by the public firewall")
		}
	}
	if public != 0 || private != 2 {
		t.Fatal("public endpoints or the single TCP/UDP policy bridge are missing")
	}
}

func clientPort(t *testing.T, output, name string) string {
	t.Helper()
	var config model.Object
	must(t, safeio.LoadJSON(filepath.Join(output, "clients", name+".json"), &config))
	for _, raw := range config["outbounds"].([]any) {
		out := raw.(map[string]any)
		if out["type"] == "tuic" {
			return fmt.Sprint(out["server_port"])
		}
		if out["protocol"] == "vless" {
			settings := out["settings"].(map[string]any)
			return fmt.Sprint(settings["vnext"].([]any)[0].(map[string]any)["port"])
		}
	}
	t.Fatal("client has no configured VPN server")
	return ""
}
