package installation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/release"
	"vpn-script/internal/platform/safeio"
)

func upgradeFixture(t *testing.T) (Revision, Revision, model.NodeConfig, string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	old := model.NodeConfig{SchemaVersion: 2, Generation: strings.Repeat("a", 64), Programs: []model.Program{{Name: "xray"}, {Name: "sing-box"}}}
	directory := filepath.Join(root, old.Generation)
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"nhop-node", "processes.json", "xray", "xray.json", "sing-box", "sing-box.json", "libcronet.so"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("old-"+name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	files, err := release.Inventory(directory, old)
	if err != nil {
		t.Fatal(err)
	}
	before := Revision{Directory: directory, State: installer.State{Generation: old.Generation, Files: files}}
	sha := safeio.Hash([]byte("new-supervisor"))
	after := Revision{Directory: filepath.Join(root, strings.Repeat("b", 64)), State: installer.State{
		PreviousReleaseID: filepath.Base(directory), PreviousGeneration: old.Generation, Files: map[string]string{"nhop-node": sha}}}
	return before, after, old, sha
}

func runtimeFixture(t *testing.T) (map[string]evidence.Runtime, map[string]evidence.Runtime, string) {
	t.Helper()
	before, _, _, _ := upgradeFixture(t)
	current, previous := map[string]evidence.Runtime{}, map[string]evidence.Runtime{}
	for _, role := range []string{"client/xray", "server/xray", "client/sing-box", "server/sing-box"} {
		_, name, _ := strings.Cut(role, "/")
		path := filepath.Join(before.Directory, name)
		sha, err := safeio.FileHash(path)
		if err != nil {
			t.Fatal(err)
		}
		previous[role] = evidence.Runtime{Path: path, SHA256: sha}
		current[role] = evidence.Runtime{SHA256: safeio.Hash([]byte("new-" + name))}
	}
	return previous, current, before.Directory
}
