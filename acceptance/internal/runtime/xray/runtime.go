package xray

import "vpn-script/internal/core/model"

const Version = "26.10.31"

func Runtime() model.Runtime {
	return model.Runtime{Version: Version, Assets: map[string]model.Asset{
		"x86_64": {
			URL:    "https://github.com/Deliner/vpn-dep-xtls-xray-core/releases/download/nhop-xray-26.10.31-masque-fair-1/Xray-linux-amd64.zip",
			SHA256: "45dbb1f527acdaf856a536648177e5304a0a41d906c8294270a3e137b3f03fea", Size: 14697185,
		},
		"aarch64": {
			URL:    "https://github.com/Deliner/vpn-dep-xtls-xray-core/releases/download/nhop-xray-26.10.31-masque-fair-1/Xray-linux-arm64.zip",
			SHA256: "89eb8a90851a04f468d835e1302ca29f4761ef9f863da76f7f0a9712a9031471", Size: 13404059,
		},
	}}
}
