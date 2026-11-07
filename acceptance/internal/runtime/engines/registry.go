package engines

import (
	"encoding/json"
	"fmt"
	"vpn-script/internal/core/model"
	"vpn-script/internal/runtime/singbox"
	"vpn-script/internal/runtime/xray"
)

func Runtime(name string) model.Runtime {
	if name == "xray" {
		return xray.Runtime()
	}
	if name == "sing-box" {
		return singbox.Runtime()
	}
	return model.Runtime{}
}

func Arguments(name, path string, check bool) []string {
	if name == "sing-box" {
		if check {
			return []string{"check", "-c", path}
		}
		return []string{"run", "-c", path}
	}
	if check {
		return []string{"run", "-test", "-config", path}
	}
	return []string{"run", "-config", path}
}

func Object(value any) model.Object {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("compiler produced non-JSON value: %v", err))
	}
	var result model.Object
	if err = json.Unmarshal(raw, &result); err != nil {
		panic(err)
	}
	return result
}

func Programs(data model.NodeConfig) []model.Program {
	if data.SchemaVersion == 1 {
		return []model.Program{{Name: "xray", Runtime: data.Runtime, Config: Object(data.Xray)}}
	}
	return data.Programs
}
