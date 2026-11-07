package singbox

import "vpn-script/internal/core/model"

const Version = "1.14.4"

func Runtime() model.Runtime {
	prefix := "https://github.com/SagerNet/sing-box/releases/download/v" + Version + "/sing-box-" + Version
	return model.Runtime{Version: Version, Assets: map[string]model.Asset{
		"x86_64":  {URL: prefix + "-linux-amd64.tar.gz", Size: 31710753, SHA256: "699a1b510aa4396b2176d32d1181920399ebf1e267835da6f3b598d0f7c24968"},
		"aarch64": {URL: prefix + "-linux-arm64.tar.gz", Size: 29140599, SHA256: "436441f76ee2f7fe21b5b2d426d3cb261a480455a704865789ad533e4273ec26"},
	}}
}
