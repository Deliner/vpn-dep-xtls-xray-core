package xray

import "vpn-script/internal/core/model"

const Version = "26.10.31"

func Runtime() model.Runtime {
	return model.Runtime{Version: Version, Assets: map[string]model.Asset{
		"x86_64": {
			URL:    "https://github.com/XTLS/Xray-core/releases/download/v26.10.31/Xray-linux-64.zip",
			SHA256: "e4bd9119f50ae7c41658b1abd0679ae06f98bd864a2907ff33f2f2cf7081b214", Size: 16692690,
		},
		"aarch64": {
			URL:    "https://github.com/XTLS/Xray-core/releases/download/v26.10.31/Xray-linux-arm64-v8a.zip",
			SHA256: "1811c8f1da0c3b6dc707ebaf1c8390e987ba39859a8f0a7aadd8526157f2ec8d", Size: 15175093,
		},
	}}
}
