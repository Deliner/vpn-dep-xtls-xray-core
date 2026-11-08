package acceptance

import (
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
