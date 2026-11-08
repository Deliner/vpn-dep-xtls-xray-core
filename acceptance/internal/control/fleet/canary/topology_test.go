package canary

import (
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/engines"
)

func (f *fixture) topologyFixture(t *testing.T) {
	t.Helper()
	f.topology = &model.Topology{Definition: model.Definition{Name: "fleet-canary", Nodes: map[string]*model.Node{}, Routes: map[string]*model.Route{}},
		Services: map[string]model.Endpoint{}, Successors: map[string]string{}}
	f.secrets = model.Secrets{Services: map[string]model.Secret{}}
	for _, nodeID := range nodes {
		node := &model.Node{Address: f.addresses[nodeID], Firewall: "preserve", BridgePortStart: 29000, Services: map[string]*model.Service{}}
		f.topology.Nodes[nodeID] = node
		for index, protocol := range []string{"shadowsocks", "tuic"} {
			service := &model.Service{Protocol: protocol, Listen: node.Address, Port: 24001 + index}
			if protocol == "tuic" {
				service.ServerName = "vpn.example.invalid"
			}
			name := nodeID + "." + protocol
			node.Services[protocol] = service
			f.topology.Services[name] = model.Endpoint{Node: nodeID, ID: protocol, Service: *service}
			secret, err := credentials.NewSecret(protocol)
			require(t, err)
			require(t, credentials.EnsureTLS(*service, &secret))
			f.secrets.Services[name] = secret
		}
	}
	for index, protocol := range []string{"shadowsocks", "tuic"} {
		path := []string{"fleet-a." + protocol, "fleet-b." + protocol, "fleet-c." + protocol}
		f.topology.Successors[path[0]], f.topology.Successors[path[1]] = path[1], path[2]
		f.topology.Routes[protocol] = &model.Route{Path: path, Client: model.Client{SOCKSPort: 25001 + index}}
	}
}

func (f *fixture) configuration(t *testing.T, variant string) map[string]model.NodeConfig {
	t.Helper()
	configs := map[string]model.NodeConfig{}
	for _, node := range nodes {
		data, err := engines.Node(f.topology, node, f.secrets)
		require(t, err)
		for index, program := range data.Programs {
			config := engines.Object(program.Config)
			if node == "fleet-c" {
				allowProbeOrigins(config, program.Name)
			}
			if variant != "initial" {
				if program.Name == "xray" {
					config["log"] = model.Object{"loglevel": "error"}
				} else {
					config["log"] = model.Object{"level": "error", "timestamp": variant == "cancel"}
					if variant == "recovery-odd" || variant == "recovery-even" {
						config["log"] = model.Object{"level": "debug", "timestamp": variant == "recovery-odd"}
					}
					if variant == "broken" && node == "fleet-c" {
						config["log"] = model.Object{"level": "info", "output": f.remote + "/denied-vpn.log"}
					}
				}
			}
			data.Programs[index].Config = config
		}
		data.Generation = ""
		data.Generation, err = safeio.Digest(data)
		require(t, err)
		require(t, manifest.Validate(data))
		configs[node] = data
	}
	return configs
}

func allowProbeOrigins(config model.Object, engine string) {
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
	if engine == "sing-box" {
		for _, raw := range config["route"].(map[string]any)["rules"].([]any) {
			rule := raw.(map[string]any)
			if rule["outbound"] == "exit" {
				rule["override_address"] = "127.0.0.1"
			}
		}
	}
}
