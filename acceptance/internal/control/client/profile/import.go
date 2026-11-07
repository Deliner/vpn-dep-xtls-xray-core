package profile

import (
	"fmt"
	"path/filepath"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

// Import consumes the generated client-profiles.json and sibling clients directory.
// Matching hashes detect mismatch, not authorship: import only a trusted export.
func Import(directory, route string) (Imported, error) {
	if err := validate.Name(route); err != nil {
		return Imported{}, err
	}
	var profiles model.Profiles
	if err := safeio.LoadJSON(filepath.Join(directory, "client-profiles.json"), &profiles); err != nil {
		return Imported{}, err
	}
	metadata, exists := profiles.Routes[route]
	if profiles.SchemaVersion != model.ClientProfilesVersion || !exists || validate.Name(profiles.TopologyID) != nil {
		return Imported{}, fmt.Errorf("unknown route or invalid client export; Linux client requires schema 2, render again")
	}
	result := Imported{SchemaVersion: 1, Topology: profiles.TopologyID, Route: route, Metadata: metadata}
	if err := safeio.LoadJSON(filepath.Join(directory, "clients", route+".json"), &result.Config); err != nil {
		return Imported{}, err
	}
	return result, Validate(result)
}

func Load(path string) (Imported, error) {
	var result Imported
	if err := safeio.LoadJSON(path, &result); err != nil {
		return result, err
	}
	return result, Validate(result)
}
