package bundle

import (
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/credentials"
)

func loadSecrets(topology *model.Topology, directory string) (model.Secrets, error) {
	state := model.Secrets{SchemaVersion: 1, TopologyID: topology.Name, Services: map[string]model.Secret{}}
	path := filepath.Join(directory, "state.json")
	if err := safeio.LoadJSON(path, &state); err != nil && !os.IsNotExist(err) {
		return state, err
	}
	if state.SchemaVersion != 1 || state.TopologyID != topology.Name || state.Services == nil {
		return state, fmt.Errorf("output state belongs to another topology/version or is invalid")
	}
	for _, name := range dag.Keys(topology.Services) {
		protocol := topology.Services[name].Service.Protocol
		secret, exists := state.Services[name]
		if exists && secret.Protocol != protocol {
			return state, fmt.Errorf("%s: protocol changed; use a new service ID", name)
		}
		var err error
		if !exists {
			secret, err = credentials.NewSecret(protocol)
		}
		if err != nil {
			return state, err
		}
		if err = credentials.ValidateSecret(secret); err != nil {
			return state, err
		}
		if secret.BridgePassword == "" {
			secret.BridgePassword = credentials.Password()
		}
		if secret.ObfsPassword == "" {
			secret.ObfsPassword = credentials.Password()
		}
		if err = credentials.EnsureTLS(topology.Services[name].Service, &secret); err != nil {
			return state, fmt.Errorf("%s: %w", name, err)
		}
		state.Services[name] = secret
	}
	return state, safeio.JSON(path, state)
}
