package engines

import (
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
	"vpn-script/internal/runtime/singbox"
	"vpn-script/internal/runtime/xray"
)

func Client(topology *model.Topology, name string, state model.Secrets) (model.Object, model.Profile) {
	route := topology.Routes[name]
	first := route.Path[0]
	endpoint := topology.Services[first]
	engine := protocol.Engine(endpoint.Service.Protocol)
	spec, _ := protocol.Lookup(endpoint.Service.Protocol)
	profile := model.Profile{Engine: engine, RuntimeVersion: Runtime(engine).Version, Path: route.Path,
		EntryEndpoint: model.ClientEndpoint{Address: topology.Nodes[endpoint.Node].Address, Port: endpoint.Service.Port, Network: spec.Network},
		HopCount:      len(route.Path), EntryNode: endpoint.Node, ExitNode: topology.Services[route.Path[len(route.Path)-1]].Node,
		ExpectedExitIP: route.ExpectedExitIP, SOCKSPort: route.Client.SOCKSPort, UDP443: "blocked", DNSPolicy: "SOCKS remote DNS; OS DNS requires separate client configuration",
		ImportNote: "Use the generated native JSON; it includes certificate trust and transport settings."}
	address, secret := topology.Nodes[endpoint.Node].Address, state.Services[first]
	if engine == "sing-box" {
		return Object(singbox.Client(singbox.Outbounds(first, address, endpoint.Service, secret), route.Client.SOCKSPort)), profile
	}
	if endpoint.Service.Protocol == model.Reality || endpoint.Service.Protocol == model.Shadowsocks || endpoint.Service.Protocol == "shadowsocks" {
		profile.URI = xray.URI(topology, first, secret, name)
	}
	return Object(xray.Client(xray.Outbound(topology, first, secret), route.Client.SOCKSPort)), profile
}
