package topology_test

import (
	"strings"
	"testing"
	"vpn-script/internal/control/topology"
	"vpn-script/internal/core/model"
)

func protocolDefinition() model.Definition {
	return model.Definition{Version: 1, Name: "protocols", Nodes: map[string]*model.Node{"node": {
		Address: "192.0.2.1", SSH: model.SSH{User: "root", IdentityFile: "~/.ssh-custom/key", KnownHostsFile: "~/.ssh-custom/known_hosts"},
		Services: map[string]*model.Service{
			"tcp": {Protocol: model.Reality, Port: 443, ServerName: "donor.example"},
			"udp": {Protocol: "tuic", Port: 443, ServerName: "vpn.example"},
		},
	}}, Routes: map[string]*model.Route{"tcp": {Path: []string{"node.tcp"}}, "udp": {Path: []string{"node.udp"}}}}
}

func TestSamePortDifferentNetworksAndRejectedConflicts(t *testing.T) {
	definition := protocolDefinition()
	if _, err := topology.Build(definition); err != nil {
		t.Fatal(err)
	}
	definition.Nodes["node"].Services["udp"].Protocol = "trojan"
	if _, err := topology.Build(definition); err == nil {
		t.Fatal("same TCP socket collision accepted")
	}
}

func TestResearchOnlyProtocolExplainsMissingPrerequisite(t *testing.T) {
	definition := protocolDefinition()
	definition.Nodes["node"].Services["udp"].Protocol = "wireguard"
	_, err := topology.Build(definition)
	if err == nil || !strings.Contains(err.Error(), "research-only") || !strings.Contains(err.Error(), "L3 adapter") {
		t.Fatal("research protocol did not report its implementation prerequisite")
	}
}

func TestTLSAndDonorOptionsAreNotSilentlyIgnored(t *testing.T) {
	for _, change := range []func(*model.Service){
		func(s *model.Service) { s.Path = "/invalid" },
		func(s *model.Service) { s.TLS.Certificate = "certificate.pem" },
		func(s *model.Service) { s.TLS.CA = "ca.pem" },
	} {
		definition := protocolDefinition()
		change(definition.Nodes["node"].Services["tcp"])
		if _, err := topology.Build(definition); err == nil {
			t.Fatal("inapplicable donor option accepted")
		}
	}
}

func TestUDPListenersCanShareNumbersWithTCPSSHChecks(t *testing.T) {
	for _, port := range []int{22, 22222} {
		definition := protocolDefinition()
		definition.Nodes["node"].Services["udp"].Port = port
		if _, err := topology.Build(definition); err != nil {
			t.Fatalf("UDP listener conflicts with TCP-only SSH contract on %d: %v", port, err)
		}
	}
}

func TestIPv4MappedListenerAliasCollidesWithIPv4(t *testing.T) {
	definition := protocolDefinition()
	definition.Nodes["node"].Services["tcp"].Listen = "127.0.0.1"
	other := definition.Nodes["node"].Services["udp"]
	other.Protocol, other.Listen = "trojan", "::ffff:127.0.0.1"
	if _, err := topology.Build(definition); err == nil {
		t.Fatal("IPv4-mapped alias accepted for the same TCP socket")
	}
}

func TestTCPAndCombinedTransportsStillCannotUseSSHNumbers(t *testing.T) {
	for _, protocol := range []string{model.Reality, "trojan", model.Shadowsocks} {
		for _, port := range []int{22, 22222} {
			definition := protocolDefinition()
			service := definition.Nodes["node"].Services["tcp"]
			service.Protocol, service.Port = protocol, port
			if protocol == model.Shadowsocks {
				service.ServerName = ""
			}
			if _, err := topology.Build(definition); err == nil || !strings.Contains(err.Error(), "SSH") {
				t.Fatalf("%s accepted TCP SSH port %d: %v", protocol, port, err)
			}
		}
	}
}
