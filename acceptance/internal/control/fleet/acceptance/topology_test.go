package acceptance

import (
	"net"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/engines"
)

func (f *fixture) topologyFixture(t *testing.T) {
	t.Helper()
	node := &model.Node{Address: "127.0.0.1", Firewall: "preserve", BridgePortStart: 29000, Services: map[string]*model.Service{}}
	f.topology = &model.Topology{Definition: model.Definition{Name: "fleet-acceptance", Nodes: map[string]*model.Node{nodeID: node}, Routes: map[string]*model.Route{}},
		Services: map[string]model.Endpoint{}, Successors: map[string]string{}}
	f.secrets = model.Secrets{Services: map[string]model.Secret{}}
	for _, protocol := range []string{"shadowsocks", "tuic"} {
		service := &model.Service{Protocol: protocol, Listen: node.Address, Port: freePort(t)}
		if protocol == "tuic" {
			service.ServerName = "vpn.example.invalid"
		}
		name := nodeID + "." + protocol
		node.Services[protocol] = service
		f.topology.Services[name] = model.Endpoint{Node: nodeID, ID: protocol, Service: *service}
		f.topology.Routes[protocol] = &model.Route{Path: []string{name}, Client: model.Client{SOCKSPort: freePort(t)}}
		secret, err := credentials.NewSecret(protocol)
		require(t, err)
		require(t, credentials.EnsureTLS(*service, &secret))
		f.secrets.Services[name] = secret
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	tcp, err := net.Listen("tcp4", "127.0.0.1:0")
	require(t, err)
	defer tcp.Close()
	port := tcp.Addr().(*net.TCPAddr).Port
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	require(t, err)
	defer udp.Close()
	return port
}

func (f *fixture) configuration(t *testing.T, changePorts bool) model.NodeConfig {
	t.Helper()
	if changePorts {
		for name, endpoint := range f.topology.Services {
			endpoint.Service.Port = freePort(t)
			f.topology.Services[name] = endpoint
			f.topology.Nodes[nodeID].Services[endpoint.ID].Port = endpoint.Service.Port
		}
	}
	data, err := engines.Node(f.topology, nodeID, f.secrets)
	require(t, err)
	for index, program := range data.Programs {
		data.Programs[index].Config = loopbackOrigins(program)
	}
	return digestConfiguration(t, data)
}

func loopbackOrigins(program model.Program) model.Object {
	config := engines.Object(program.Config)
	for _, raw := range config["outbounds"].([]any) {
		out := raw.(map[string]any)
		if out["protocol"] == "freedom" {
			settings := engines.Object(out["settings"])
			rules, _ := settings["finalRules"].([]any)
			settings["redirect"] = "127.0.0.1:0"
			settings["finalRules"] = append(rules, model.Object{"action": "allow", "ip": []string{"127.0.0.1/32"}, "network": "tcp,udp"})
			out["settings"] = settings
		}
	}
	if program.Name == "sing-box" {
		for _, raw := range config["route"].(map[string]any)["rules"].([]any) {
			rule := raw.(map[string]any)
			if rule["outbound"] == "exit" {
				rule["override_address"] = "127.0.0.1"
			}
		}
	}
	return config
}

func digestConfiguration(t *testing.T, data model.NodeConfig) model.NodeConfig {
	t.Helper()
	data.Generation = ""
	var err error
	data.Generation, err = safeio.Digest(data)
	require(t, err)
	require(t, manifest.Validate(data))
	return data
}
