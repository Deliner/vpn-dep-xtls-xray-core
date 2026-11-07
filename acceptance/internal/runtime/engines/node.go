package engines

import (
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

func Node(topology *model.Topology, nodeID string, state model.Secrets) (model.NodeConfig, error) {
	return NewCompiler(topology).Node(nodeID, state)
}

func (c *Compiler) Node(nodeID string, state model.Secrets) (model.NodeConfig, error) {
	topology := c.topology
	if c.onlyXray {
		return xray.Node(topology, nodeID, c.services[nodeID], state)
	}
	data := model.NodeConfig{SchemaVersion: 2, TopologyID: topology.Name, NodeID: nodeID, Firewall: topology.Nodes[nodeID].Firewall}
	g := newGraph(topology.Nodes[nodeID])
	for _, name := range c.services[nodeID] {
		endpoint := topology.Services[name]
		if endpoint.Node != nodeID {
			continue
		}
		engine := protocol.Engine(endpoint.Service.Protocol)
		if err := g.inbound(engine, name, endpoint.Service, state.Services[name], false); err != nil {
			return data, err
		}
		if err := connect(g, topology, name, engine, state); err != nil {
			return data, err
		}
	}
	data.Programs, data.Listeners = g.programs(), g.listeners
	var err error
	data.Generation, err = safeio.Digest(data)
	return data, err
}

func connect(g *graph, topology *model.Topology, name, engine string, state model.Secrets) error {
	next := topology.Successors[name]
	if next == "" {
		return g.exit(engine, name, state.Services[name])
	}
	target := topology.Services[next]
	address := ""
	if next != "" {
		address = topology.Nodes[target.Node].Address
	}
	if protocol.Engine(target.Service.Protocol) == engine {
		g.route(engine, name, g.outbound(engine, next, address, target.Service, state.Services[next]))
		return nil
	}
	port, err := g.port()
	if err != nil {
		return err
	}
	bridgeName := "bridge-" + name
	bridge := model.Service{Protocol: model.Shadowsocks, Listen: "127.0.0.1", Port: port}
	key := model.Secret{Protocol: model.Shadowsocks, Password: state.Services[name].BridgePassword}
	other := protocol.Engine(target.Service.Protocol)
	if err := g.inbound(other, bridgeName, bridge, key, true); err != nil {
		return err
	}
	g.route(other, bridgeName, g.outbound(other, next, address, target.Service, state.Services[next]))
	g.route(engine, name, g.outbound(engine, bridgeName, "127.0.0.1", bridge, key))
	return nil
}
