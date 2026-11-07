package manifest

import (
	"fmt"

	"reflect"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

func Load(path string) (model.NodeConfig, error) {
	var data model.NodeConfig
	if err := safeio.LoadJSON(path, &data); err != nil {
		return data, err
	}
	return data, Validate(data)
}

func Validate(data model.NodeConfig) error {
	if (data.SchemaVersion != 1 && data.SchemaVersion != 2) || validate.Name(data.NodeID) != nil || validate.Name(data.TopologyID) != nil {
		return fmt.Errorf("invalid node identity/schema")
	}
	generation := data.Generation
	data.Generation = ""
	hash, err := safeio.Digest(data)
	if err != nil || !validate.Hash(generation) || hash != generation {
		return fmt.Errorf("node config checksum mismatch; regenerate the bundle")
	}
	if data.SchemaVersion == 1 && !reflect.DeepEqual(data.Runtime, xray.Runtime()) {
		return fmt.Errorf("runtime must match the pinned Xray release, URLs, sizes and digests")
	}
	if data.Firewall != "preserve" && data.Firewall != "ufw" {
		return fmt.Errorf("invalid firewall mode")
	}
	if data.SchemaVersion == 1 && (len(data.Xray.Outbounds) == 0 || !reflect.DeepEqual(data.Xray.Outbounds[0], xray.Block())) {
		return fmt.Errorf("the default outbound must be block")
	}
	if data.SchemaVersion == 1 && len(data.Programs) != 0 {
		return fmt.Errorf("v1 manifest cannot contain programs")
	}
	if data.SchemaVersion == 2 && (len(data.Programs) == 0 || !reflect.DeepEqual(data.Xray, model.Xray{}) || !reflect.DeepEqual(data.Runtime, model.Runtime{})) {
		return fmt.Errorf("invalid v2 engine envelope")
	}
	return validateListeners(data)
}
