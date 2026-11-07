package engines

import (
	"slices"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
)

// NewCompiler indexes each service once, preserving the original lexical order
// within every node and the topology-wide choice of manifest schema.
func NewCompiler(topology *model.Topology) *Compiler {
	compiler := &Compiler{topology: topology, services: map[string][]string{}, onlyXray: true}
	for name, endpoint := range topology.Services {
		compiler.services[endpoint.Node] = append(compiler.services[endpoint.Node], name)
		compiler.onlyXray = compiler.onlyXray && protocol.Engine(endpoint.Service.Protocol) == "xray"
	}
	for _, names := range compiler.services {
		slices.Sort(names)
	}
	return compiler
}
