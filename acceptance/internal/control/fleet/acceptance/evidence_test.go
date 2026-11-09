package acceptance

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/release"
	"vpn-script/internal/platform/safeio"
)

func (f *fixture) verifyInstallation(t *testing.T, data model.NodeConfig) {
	t.Helper()
	var state installer.State
	require(t, safeio.LoadJSON(filepath.Join(nodeBase, "state.json"), &state))
	directory, err := filepath.EvalSymlinks(filepath.Join(nodeBase, "current"))
	require(t, err)
	files, err := release.Inventory(directory, data)
	require(t, err)
	if state.Generation != data.Generation || filepath.Base(directory) != state.ReleaseID || !reflect.DeepEqual(files, state.Files) {
		t.Fatal("installed generation, release and file inventory disagree")
	}
	hash, err := safeio.FileHash(f.binary)
	require(t, err)
	if files["nhop-node"] != hash || property(t, "vpn-nhop.service", "SubState") != "running" {
		t.Fatal("real systemd VPN service is not using the Fleet executable as supervisor")
	}
	f.report.AgentSHA256, f.report.SupervisorSHA256 = hash, files["nhop-node"]
	f.report.RuntimeSHA256 = map[string]string{"xray": files["xray"], "sing-box": files["sing-box"], "libcronet.so": files["libcronet.so"]}
	f.retainRuntimes(t, directory, files)
}

func (f *fixture) retainRuntimes(t *testing.T, directory string, files map[string]string) {
	t.Helper()
	destination := filepath.Join(f.root, "reports", "runtime")
	require(t, os.MkdirAll(destination, 0700))
	for _, name := range []string{"xray", "sing-box", "libcronet.so"} {
		data, err := safeio.Read(filepath.Join(directory, name), 256<<20)
		require(t, err)
		if safeio.Hash(data) != files[name] {
			t.Fatal("installed runtime changed while retaining evidence", name)
		}
		target := filepath.Join(destination, name)
		if previous, err := os.ReadFile(target); err == nil {
			if !bytes.Equal(previous, data) {
				t.Fatal("Fleet transaction changed pinned runtime bytes", name)
			}
			continue
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		require(t, os.WriteFile(target, data, 0600))
	}
}

func (f *fixture) saveReport(t *testing.T) {
	t.Helper()
	f.report.Status = "passed"
	if t.Failed() {
		f.report.Status = "failed"
	}
	if err := safeio.JSON(os.Getenv("NHOP_FLEET_REPORT"), f.report); err != nil {
		t.Error(err)
	}
}
