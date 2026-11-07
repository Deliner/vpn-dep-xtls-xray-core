package bundle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"vpn-script/internal/control/bundle"
	"vpn-script/internal/control/topology"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
)

func TestMultiEngineBundleAndPrivateKeyDistribution(t *testing.T) {
	_, output, installers := fixture(t)
	top, err := topology.Load("../../../examples/nhop/protocols.yaml")
	must(t, err)
	output = filepath.Join(filepath.Dir(output), "matrix")
	must(t, bundle.Render(top, output, installers))
	_, err = bundle.Load(output)
	must(t, err)
	before, err := os.ReadFile(filepath.Join(output, "state.json"))
	must(t, err)
	must(t, bundle.Render(top, output, installers))
	after, err := os.ReadFile(filepath.Join(output, "state.json"))
	must(t, err)
	if string(before) != string(after) {
		t.Fatal("rerender rotated secrets or certificates")
	}
	var state model.Secrets
	must(t, safeio.LoadJSON(filepath.Join(output, "state.json"), &state))
	for id := range top.Nodes {
		checkProgramSecrets(t, top, output, id, state)
	}
	for name := range top.Routes {
		raw, err := os.ReadFile(filepath.Join(output, "clients", name+".json"))
		must(t, err)
		for _, secret := range state.Services {
			assertAbsent(t, raw, secret.PrivateKey, secret.CertificateKey, secret.EncryptionSeed)
		}
	}
}

func checkProgramSecrets(t *testing.T, top *model.Topology, output, id string, state model.Secrets) {
	t.Helper()
	path := filepath.Join(output, "nodes", id+".json")
	data, err := manifest.Load(path)
	must(t, err)
	if data.SchemaVersion != 2 || len(data.Programs) != 2 {
		t.Fatal("missing engine processes")
	}
	raw, err := os.ReadFile(path)
	must(t, err)
	for name, secret := range state.Services {
		if top.Services[name].Node != id {
			assertAbsent(t, raw, secret.PrivateKey, secret.CertificateKey, secret.EncryptionSeed)
		}
	}
	for _, listener := range data.Listeners {
		if listener.Internal && strings.Contains(installer.UFWProfile(data), fmt.Sprintf("%d/%s", listener.Port, listener.Network)) {
			t.Fatal("internal listener exposed")
		}
	}
	data.Programs[0].Runtime.Version = "untrusted"
	data.Generation = ""
	data.Generation, err = safeio.Digest(data)
	must(t, err)
	if manifest.Validate(data) == nil {
		t.Fatal("unpinned runtime accepted")
	}
}

func assertAbsent(t *testing.T, raw []byte, secrets ...string) {
	t.Helper()
	for _, secret := range secrets {
		// PEM newlines are escaped by JSON; compare a nonempty base64 body line too.
		if strings.Contains(secret, "\n") {
			secret = strings.Split(secret, "\n")[1]
		}
		if secret != "" && strings.Contains(string(raw), secret) {
			t.Fatal("server private key distributed outside its owning node")
		}
	}
}
