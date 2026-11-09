// Package previous contains immutable runtime fixtures from project 4744ac5.
// These are acceptance inputs for the previous installer, never production pins.
package previous

import (
	"fmt"

	"vpn-script/internal/core/model"
)

const Project = "4744ac5a164f33a1d6a2958c5d17073b69ca4747"

func Runtime(engine string) (model.Runtime, error) {
	switch engine {
	case "xray":
		prefix := "https://github.com/Deliner/vpn-dep-xtls-xray-core/releases/download/nhop-xray-26.10.31-masque-fair-1/Xray-linux-"
		return model.Runtime{Version: "26.10.31", Assets: map[string]model.Asset{
			"x86_64": {URL: prefix + "amd64.zip", Size: 14697185,
				SHA256: "45dbb1f527acdaf856a536648177e5304a0a41d906c8294270a3e137b3f03fea"},
			"aarch64": {URL: prefix + "arm64.zip", Size: 13404059,
				SHA256: "89eb8a90851a04f468d835e1302ca29f4761ef9f863da76f7f0a9712a9031471"},
		}}, nil
	case "sing-box":
		prefix := "https://github.com/SagerNet/sing-box/releases/download/v1.14.4/sing-box-1.14.4-linux-"
		return model.Runtime{Version: "1.14.4", Assets: map[string]model.Asset{
			"x86_64": {URL: prefix + "amd64.tar.gz", Size: 31710753,
				SHA256: "699a1b510aa4396b2176d32d1181920399ebf1e267835da6f3b598d0f7c24968"},
			"aarch64": {URL: prefix + "arm64.tar.gz", Size: 29140599,
				SHA256: "436441f76ee2f7fe21b5b2d426d3cb261a480455a704865789ad533e4273ec26"},
		}}, nil
	}
	return model.Runtime{}, fmt.Errorf("no previous fixture for engine %q", engine)
}
