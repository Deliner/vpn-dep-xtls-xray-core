package xray

import "vpn-script/internal/core/model"

const Version = "26.10.31"

func Runtime() model.Runtime {
	return model.Runtime{Version: Version, Assets: map[string]model.Asset{
		"x86_64": {
			URL:    "https://github.com/Deliner/vpn-dep-xtls-xray-core/releases/download/nhop-xray-26.10.31-masque-sack-1/Xray-linux-amd64.zip",
			SHA256: "6871a4369d09baaaba40043af2123a10dcf7002cf63ec8792b0257b359887546", Size: 14733965,
		},
		"aarch64": {
			URL:    "https://github.com/Deliner/vpn-dep-xtls-xray-core/releases/download/nhop-xray-26.10.31-masque-sack-1/Xray-linux-arm64.zip",
			SHA256: "ada2725c842e2a72b56f104653035bb3bfaa09b74f7ec2c4b0cc7bbb65da890b", Size: 13437102,
		},
	}}
}
