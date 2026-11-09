package singbox

import "vpn-script/internal/core/model"

const Version = "1.14.4"

func Runtime() model.Runtime {
	prefix := "https://github.com/Deliner/vpn-dep-sagernet-sing-box/releases/download/nhop-sing-box-" + Version + "-cronet-zlib-tuic-1/sing-box-" + Version
	return model.Runtime{Version: Version, Assets: map[string]model.Asset{
		"x86_64":  {URL: prefix + "-linux-amd64.tar.gz", Size: 34789149, SHA256: "60f6e8c902809e3baa1b1514143a9d993cc8062c81b0d31ef76433552eb6b6ce"},
		"aarch64": {URL: prefix + "-linux-arm64.tar.gz", Size: 32306554, SHA256: "da366329fd70df7638512e055697e28240c42db743a55b57a233e77c701e33ee"},
	}}
}
