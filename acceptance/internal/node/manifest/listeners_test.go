package manifest_test

import (
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
	"vpn-script/internal/runtime/xray"
)

func socketManifest(t *testing.T, schema int, first, second string) model.NodeConfig {
	t.Helper()
	data := model.NodeConfig{SchemaVersion: schema, TopologyID: "ports", NodeID: "node", Firewall: "preserve", Runtime: xray.Runtime()}
	for index, address := range []string{first, second} {
		tag := []string{"first", "second"}[index]
		data.Xray.Inbounds = append(data.Xray.Inbounds, model.Object{"protocol": "vless", "tag": tag, "listen": address, "port": 24443})
		data.Listeners = append(data.Listeners, model.Listener{Tag: tag, Address: address, Port: 24443, Network: "tcp"})
	}
	data.Xray.Outbounds = []model.Object{xray.Block()}
	if schema == 2 {
		data.Programs = []model.Program{{Name: "xray", Runtime: xray.Runtime(), Config: engines.Object(data.Xray)}}
		data.Runtime, data.Xray = model.Runtime{}, model.Xray{}
	}
	var err error
	data.Generation, err = safeio.Digest(data)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestManifestRejectsCollidingSocketBindingsRegardlessOfTags(t *testing.T) {
	for _, schema := range []int{1, 2} {
		for _, addresses := range [][2]string{{"127.0.0.1", "127.0.0.1"}, {"0.0.0.0", "127.0.0.1"}, {"127.0.0.1", "::ffff:127.0.0.1"}, {"::", "::1"}} {
			data := socketManifest(t, schema, addresses[0], addresses[1])
			if err := manifest.Validate(data); err == nil {
				t.Errorf("schema %d accepted distinct tags on conflicting addresses %v", schema, addresses)
			}
		}
	}
}

func TestManifestAllowsDisjointAddressesOnTheSameTCPPort(t *testing.T) {
	for _, schema := range []int{1, 2} {
		data := socketManifest(t, schema, "127.0.0.1", "127.0.0.2")
		if err := manifest.Validate(data); err != nil {
			t.Fatal("disjoint literal addresses collided", err)
		}
	}
}
