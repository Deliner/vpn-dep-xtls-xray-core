package singbox

import "vpn-script/internal/core/model"

const Version = "1.14.4"

func Runtime() model.Runtime {
	prefix := "https://github.com/Deliner/vpn-dep-sagernet-sing-box/releases/download/nhop-sing-box-" + Version + "-cronet-zlib-1/sing-box-" + Version
	return model.Runtime{Version: Version, Assets: map[string]model.Asset{
		"x86_64":  {URL: prefix + "-linux-amd64.tar.gz", Size: 34699400, SHA256: "41c25ed2ab8c0ef959f3202b037dec595ac9ebe400d915ae178a1cb134acbdcc"},
		"aarch64": {URL: prefix + "-linux-arm64.tar.gz", Size: 32186039, SHA256: "07e10dc68b8e17b64417212d6910e323dd23d893bd6b6bf5aa7ac49e002727ef"},
	}}
}
