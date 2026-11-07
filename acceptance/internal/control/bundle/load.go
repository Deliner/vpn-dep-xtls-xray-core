package bundle

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

func checkHash(path, expected string) error {
	actual, err := safeio.FileHash(path)
	if err != nil {
		return err
	}
	if !validate.Hash(expected) || actual != expected {
		return fmt.Errorf("bundle file hash mismatch: %s; render again", path)
	}
	return nil
}

// Load validates the files currently in the bundle. Hold Lock while using them
// if another render/deploy/verify operation could access the same directory.
func Load(directory string) (model.Deployment, error) {
	var manifest model.Deployment
	if err := safeio.LoadJSON(filepath.Join(directory, "deployment.json"), &manifest); err != nil {
		return manifest, err
	}
	if manifest.SchemaVersion != 2 || validate.Name(manifest.TopologyID) != nil || len(manifest.Nodes) == 0 {
		return manifest, fmt.Errorf("expected Go deployment schema 2; render again using the saved state")
	}
	if err := checkInstallers(directory, manifest.Installers); err != nil {
		return manifest, err
	}
	if err := checkNodes(directory, &manifest); err != nil {
		return manifest, err
	}
	return manifest, checkHash(filepath.Join(directory, "client-profiles.json"), manifest.ProfilesSHA256)
}

func checkInstallers(directory string, installers map[string]model.Binary) error {
	if len(installers) != len(installerFiles) {
		return fmt.Errorf("both Linux installer architectures are required")
	}
	for arch, name := range installerFiles {
		binary, exists := installers[arch]
		if !exists || binary.File != "installers/"+name {
			return fmt.Errorf("unexpected installer file path")
		}
		if err := checkHash(filepath.Join(directory, binary.File), binary.SHA256); err != nil {
			return err
		}
	}
	return nil
}

func checkNodes(directory string, manifest *model.Deployment) error {
	if len(manifest.Order) != len(manifest.Nodes) {
		return fmt.Errorf("invalid deployment order")
	}
	seen := map[string]bool{}
	for _, id := range manifest.Order {
		node, exists := manifest.Nodes[id]
		if validate.Name(id) != nil || !exists || seen[id] || validate.Host(node.Address) != nil {
			return fmt.Errorf("invalid deployment node/order")
		}
		seen[id] = true
		if err := validate.SSH(&node.SSH); err != nil {
			return err
		}
		manifest.Nodes[id] = node
		path := filepath.Join(directory, "nodes", id+".json")
		if err := checkHash(path, node.SHA256); err != nil {
			return err
		}
		var config model.NodeConfig
		if err := safeio.LoadJSON(path, &config); err != nil {
			return err
		}
		if config.NodeID != id || config.TopologyID != manifest.TopologyID {
			return fmt.Errorf("node configuration identity mismatch")
		}
	}
	return nil
}

// Dependencies reads the verified route paths while the caller holds Lock.
// Load must have validated manifest and its deployment order first.
func Dependencies(directory string, manifest model.Deployment) (map[string][]string, error) {
	path := filepath.Join(directory, "client-profiles.json")
	if err := checkHash(path, manifest.ProfilesSHA256); err != nil {
		return nil, err
	}
	var profiles model.Profiles
	if err := safeio.LoadJSON(path, &profiles); err != nil {
		return nil, err
	}
	if profiles.SchemaVersion != 1 || profiles.TopologyID != manifest.TopologyID || len(profiles.Routes) == 0 {
		return nil, fmt.Errorf("invalid deployment route metadata; render again")
	}
	order, edges, used := map[string]int{}, map[string]map[string]bool{}, map[string]bool{}
	for index, id := range manifest.Order {
		order[id], edges[id] = index, map[string]bool{}
	}
	for name, profile := range profiles.Routes {
		nodes, err := deploymentPath(profile, order)
		if validate.Name(name) != nil || err != nil {
			return nil, fmt.Errorf("invalid deployment route %q: %v; render again", name, err)
		}
		for index, id := range nodes {
			used[id] = true
			if index+1 < len(nodes) {
				edges[id][nodes[index+1]] = true
			}
		}
	}
	if len(used) != len(manifest.Nodes) {
		return nil, fmt.Errorf("deployment routes do not cover every node; render again")
	}
	return sortedDependencies(edges), nil
}

func deploymentPath(profile model.Profile, order map[string]int) ([]string, error) {
	if len(profile.Path) == 0 || profile.HopCount != len(profile.Path) {
		return nil, fmt.Errorf("missing or inconsistent path")
	}
	nodes := make([]string, 0, len(profile.Path))
	for _, endpoint := range profile.Path {
		node, service, found := strings.Cut(endpoint, ".")
		index, exists := order[node]
		if !found || !exists || validate.Name(node) != nil || validate.Name(service) != nil {
			return nil, fmt.Errorf("unknown or invalid path endpoint")
		}
		if len(nodes) > 0 && index >= order[nodes[len(nodes)-1]] {
			return nil, fmt.Errorf("deployment order must place downstream nodes first")
		}
		nodes = append(nodes, node)
	}
	if profile.EntryNode != nodes[0] || profile.ExitNode != nodes[len(nodes)-1] {
		return nil, fmt.Errorf("entry/exit does not match route path")
	}
	return nodes, nil
}

func sortedDependencies(edges map[string]map[string]bool) map[string][]string {
	dependencies := make(map[string][]string, len(edges))
	for id, downstream := range edges {
		dependencies[id] = []string{}
		for next := range downstream {
			dependencies[id] = append(dependencies[id], next)
		}
		slices.Sort(dependencies[id])
	}
	return dependencies
}
