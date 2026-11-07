package xray

import (
	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

// Node compiles the ordered service names selected by the fleet compiler.
func Node(topology *model.Topology, nodeID string, names []string, state model.Secrets) (model.NodeConfig, error) {
	data := model.NodeConfig{SchemaVersion: 1, TopologyID: topology.Name, NodeID: nodeID,
		Runtime: Runtime(), Firewall: topology.Nodes[nodeID].Firewall}
	data.Xray.Log = model.Object{"loglevel": "warning"}
	outs := map[string]model.Object{"block": Block()}
	rules := []model.Object{{"type": "field", "network": "udp", "port": "443", "outboundTag": "block"}}
	for _, name := range names {
		endpoint := topology.Services[name]
		if endpoint.Node != nodeID {
			continue
		}
		data.Xray.Inbounds = append(data.Xray.Inbounds, Inbound(name, endpoint.Service, state.Services[name]))
		data.Listeners = append(data.Listeners, Listeners(name, endpoint.Service)...)
		out := nextOutbound(topology, name, state)
		tag := out["tag"].(string)
		outs[tag] = out
		rules = append(rules, model.Object{"type": "field", "inboundTag": []string{name}, "network": "tcp,udp", "outboundTag": tag})
	}
	for _, tag := range dag.Keys(outs) {
		data.Xray.Outbounds = append(data.Xray.Outbounds, outs[tag])
	}
	data.Xray.Routing = model.Object{"domainStrategy": "AsIs", "rules": rules}
	var err error
	data.Generation, err = safeio.Digest(data)
	return data, err
}

func nextOutbound(topology *model.Topology, name string, state model.Secrets) model.Object {
	next := topology.Successors[name]
	if next == "" {
		return Exit()
	}
	return Outbound(topology, next, state.Services[next])
}
