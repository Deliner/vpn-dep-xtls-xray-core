package selection

import (
	"fmt"

	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/core/dag"
)

func New(nodes []string, options Options) (Set, error) {
	selected, err := updatedNodes(nodes, options.UpdatedNodes)
	if err != nil {
		return Set{}, err
	}
	if err := validateSources(options, len(selected) != 0); err != nil {
		return Set{}, err
	}
	result := Set{Inputs: map[string]evidence.Input{}, Clients: map[string]string{}, Nodes: map[string]map[string]string{}}
	for _, engine := range dag.Keys(options.Servers) {
		input, exists := options.Clients[engine]
		if !exists {
			input = options.Servers[engine]
		}
		id := "client/" + engine
		result.Inputs[id], result.Clients[engine] = input, id
	}
	for _, node := range nodes {
		role, sources := "server", options.Servers
		if selected[node] {
			role, sources = "updated", options.Updated
		}
		result.Nodes[node] = map[string]string{}
		for _, engine := range dag.Keys(options.Servers) {
			id := role + "/" + engine
			result.Inputs[id], result.Nodes[node][engine] = sources[engine], id
		}
	}
	return result, nil
}

func updatedNodes(nodes, updated []string) (map[string]bool, error) {
	known, selected := map[string]bool{}, map[string]bool{}
	for _, node := range nodes {
		if node == "" || known[node] {
			return nil, fmt.Errorf("empty or duplicate node identity %q", node)
		}
		known[node] = true
	}
	if len(known) == 0 {
		return nil, fmt.Errorf("no runtime nodes requested")
	}
	for _, node := range updated {
		if !known[node] || selected[node] {
			return nil, fmt.Errorf("unknown or duplicate updated node %q", node)
		}
		selected[node] = true
	}
	return selected, nil
}

func validateSources(options Options, update bool) error {
	if len(options.Servers) == 0 {
		return fmt.Errorf("no server runtimes requested")
	}
	for role, sources := range map[string]map[string]evidence.Input{"server": options.Servers, "client": options.Clients, "updated": options.Updated} {
		for _, engine := range dag.Keys(sources) {
			if _, exists := options.Servers[engine]; !exists || engine == "" {
				return fmt.Errorf("unknown %s runtime %q", role, engine)
			}
			if sources[engine].Path == "" || sources[engine].ExpectedVersion == "" || sources[engine].ExpectedVersion != options.Servers[engine].ExpectedVersion {
				return fmt.Errorf("missing path or mismatched expected version for %s runtime %s", role, engine)
			}
		}
	}
	if !update && len(options.Updated) != 0 {
		return fmt.Errorf("updated runtimes supplied without selected nodes")
	}
	if update && len(options.Updated) != len(options.Servers) {
		return fmt.Errorf("every engine needs an updated runtime for selected nodes")
	}
	return nil
}
