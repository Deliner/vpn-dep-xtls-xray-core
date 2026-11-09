package previous_test

import (
	"reflect"
	"testing"

	"vpn-script/internal/control/integration/evidence/installation/previous"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
	"vpn-script/internal/runtime/xray"
)

func fixture(t *testing.T, schema int) model.NodeConfig {
	t.Helper()
	data := model.NodeConfig{SchemaVersion: schema, TopologyID: "previous", NodeID: "node", Firewall: "preserve", Runtime: xray.Runtime(),
		Listeners: []model.Listener{{Tag: "in", Address: "127.0.0.1", Port: 24443, Network: "tcp"}}}
	data.Xray.Inbounds = []model.Object{{"protocol": "vless", "tag": "in", "listen": "127.0.0.1", "port": 24443}}
	data.Xray.Outbounds = []model.Object{xray.Block()}
	if schema == 2 {
		data.Programs = []model.Program{{Name: "xray", Runtime: data.Runtime, Config: engines.Object(data.Xray)}}
		data.Xray, data.Runtime = model.Xray{}, model.Runtime{}
	}
	var err error
	data.Generation, err = safeio.Digest(data)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestHistoricalManifestDoesNotLoosenCurrentPinPolicy(t *testing.T) {
	for _, schema := range []int{1, 2} {
		current := fixture(t, schema)
		if err := manifest.Validate(current); err != nil {
			t.Fatal(err)
		}
		before, err := safeio.FormattedJSON(current)
		if err != nil {
			t.Fatal(err)
		}
		old, err := previous.Manifest(current)
		if err != nil || old.Generation == current.Generation {
			t.Fatal("historical runtime identity was not selected", err)
		}
		if err := manifest.Validate(old); err == nil {
			t.Fatal("current installer accepted old compiled pins")
		}
		after, err := safeio.FormattedJSON(current)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("historical fixture mutated the current input")
		}
		generation := old.Generation
		old.Generation = ""
		digest, err := safeio.Digest(old)
		if err != nil || digest != generation {
			t.Fatal("historical manifest checksum was not recomputed")
		}
	}
}

func TestHistoricalFixtureRejectsUnknownEnginesAndSchemas(t *testing.T) {
	if _, err := previous.Runtime("shell"); err == nil {
		t.Fatal("unknown historical runtime accepted")
	}
	data := fixture(t, 2)
	data.Programs[0].Name = "shell"
	if _, err := previous.Manifest(data); err == nil {
		t.Fatal("historical manifest accepted an arbitrary engine")
	}
	data.SchemaVersion = 3
	if _, err := previous.Manifest(data); err == nil {
		t.Fatal("unknown historical schema accepted")
	}
}
