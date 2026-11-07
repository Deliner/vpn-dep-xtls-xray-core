package bundle

import (
	"path/filepath"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
)

func renderNodes(topology *model.Topology, directory string, state model.Secrets) (map[string]model.DeployNode, error) {
	nodes := map[string]model.DeployNode{}
	compiler := engines.NewCompiler(topology)
	for _, id := range topology.Order {
		payload, err := compiler.Node(id, state)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(directory, "nodes", id+".json")
		if err := safeio.JSON(path, payload); err != nil {
			return nil, err
		}
		hash, err := safeio.FileHash(path)
		if err != nil {
			return nil, err
		}
		node := topology.Nodes[id]
		nodes[id] = model.DeployNode{Address: node.Address, SSH: node.SSH, SHA256: hash}
	}
	return nodes, nil
}
