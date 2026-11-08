package canary

import (
	"path/filepath"
	"reflect"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/node/release"
	"vpn-script/internal/platform/safeio"
)

func TestFleetGuestInventory(t *testing.T) {
	root := guestRoot(t)
	var expected model.NodeConfig
	require(t, safeio.LoadJSON(filepath.Join(root, "expected.json"), &expected))
	require(t, manifest.Validate(expected))
	var state installer.State
	require(t, safeio.LoadJSON(filepath.Join(nodeBase, "state.json"), &state))
	directory, err := filepath.EvalSymlinks(filepath.Join(nodeBase, "current"))
	require(t, err)
	files, err := release.Inventory(directory, expected)
	require(t, err)
	hash, err := safeio.FileHash(filepath.Join(root, "nhop-fleet"))
	require(t, err)
	if state.Generation != expected.Generation || filepath.Base(directory) != state.ReleaseID || !reflect.DeepEqual(files, state.Files) || files["nhop-node"] != hash {
		t.Fatal("installed files, executable, release and generation differ from their receipts")
	}
}
