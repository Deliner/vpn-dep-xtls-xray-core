package previous

import (
	"fmt"
	"slices"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

// Manifest substitutes only reviewed historical runtime identities. The real
// previous installer remains responsible for accepting this manifest and pins.
func Manifest(data model.NodeConfig) (model.NodeConfig, error) {
	var err error
	switch data.SchemaVersion {
	case 1:
		data.Runtime, err = Runtime("xray")
	case 2:
		data.Programs = slices.Clone(data.Programs)
		for index := range data.Programs {
			data.Programs[index].Runtime, err = Runtime(data.Programs[index].Name)
			if err != nil {
				return data, err
			}
		}
	default:
		return data, fmt.Errorf("unsupported previous manifest schema")
	}
	if err != nil {
		return data, err
	}
	data.Generation = ""
	data.Generation, err = safeio.Digest(data)
	return data, err
}
