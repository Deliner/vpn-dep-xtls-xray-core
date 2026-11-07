package bundle

import (
	"path/filepath"
	"strings"

	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/platform/sshutil"
	"vpn-script/internal/runtime/engines"
)

func renderClient(topology *model.Topology, name, directory string, state model.Secrets) (model.Profile, error) {
	config, profile := engines.Client(topology, name, state)
	hash, err := safeio.Digest(config)
	if err != nil {
		return model.Profile{}, err
	}
	profile.ConfigSHA256 = hash
	err = safeio.JSON(filepath.Join(directory, "clients", name+".json"), config)
	return profile, err
}

func renderClients(topology *model.Topology, directory string, state model.Secrets) (string, error) {
	profiles := model.Profiles{SchemaVersion: 1, TopologyID: topology.Name, Routes: map[string]model.Profile{}}
	var lines []string
	for _, name := range topology.Order {
		node := topology.Nodes[name]
		connection := model.DeployNode{Address: node.Address, SSH: node.SSH}
		lines = append(lines, name+": "+sshutil.Shell(sshutil.Connect(connection)...))
	}
	for _, name := range dag.Keys(topology.Routes) {
		profile, err := renderClient(topology, name, directory, state)
		if err != nil {
			return "", err
		}
		profiles.Routes[name] = profile
		lines = append(lines, "", name+": "+profile.URI, "  engine="+profile.Engine+" "+profile.RuntimeVersion+"; clients/"+name+".json", "  "+profile.ImportNote)
	}
	path := filepath.Join(directory, "client-profiles.json")
	if err := safeio.JSON(path, profiles); err != nil {
		return "", err
	}
	if err := safeio.Atomic(filepath.Join(directory, "handoff.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
		return "", err
	}
	return safeio.FileHash(path)
}
