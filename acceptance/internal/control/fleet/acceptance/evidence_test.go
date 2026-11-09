package acceptance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/fleet/service"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/release"
	"vpn-script/internal/platform/fleetapi"
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

func (f *fixture) controllerSnapshot(t *testing.T, name string, revoked wire.Enrollment) controllerSnapshot {
	t.Helper()
	c := f.controller
	value := controllerSnapshot{PID: c.child.Process.Pid, Hashes: map[string]string{},
		AgentPID: property(t, service.Name, "MainPID"), VPNPID: property(t, "vpn-nhop.service", "MainPID")}
	var err error
	value.Active, err = c.client.Node(t.Context(), nodeID, c.admin)
	require(t, err)
	value.Revoked, err = c.client.Node(t.Context(), revoked.NodeID, c.admin)
	require(t, err)
	value.RevokedAssignmentHTTP, value.RevokedReportHTTP, value.RevokedEnrollHTTP = c.denied(t, revoked)
	for key, path := range map[string]string{"authority": filepath.Join(c.root, "authority.json"), "admin": filepath.Join(c.root, "admin-credential.json"),
		"tls_certificate": filepath.Join(c.root, "ca.pem"), "tls_key": filepath.Join(c.root, "key.pem"), "agent_credentials": filepath.Join(agentState, "connection.json")} {
		value.Hashes[key], err = safeio.FileHash(path)
		require(t, err)
	}
	executable, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", value.PID))
	require(t, err)
	if executable != f.binary {
		t.Fatal("controller process is not executing the supplied Fleet binary")
	}
	value.Hashes["executable"], err = safeio.FileHash(executable)
	require(t, err)
	value.AssignmentSHA256 = f.controllerAssignment(t, name, value.Active)
	require(t, safeio.NewJSON(filepath.Join(f.root, "reports", "controller-"+name+".json"), value))
	return value
}

func (f *fixture) controllerAssignment(t *testing.T, name string, node wire.Node) string {
	t.Helper()
	var connection struct {
		Credential string         `json:"credential"`
		Trust      contract.Trust `json:"trust"`
	}
	raw, err := safeio.Read(filepath.Join(agentState, "connection.json"), 1<<20)
	require(t, err)
	require(t, json.Unmarshal(raw, &connection))
	raw, err = f.controller.client.Assignment(t.Context(), nodeID, connection.Credential)
	require(t, err)
	_, receipt, err := contract.Accept(raw, connection.Trust, contract.Receipt{}, time.Now())
	require(t, err)
	if safeio.Hash(raw) != node.Desired.EnvelopeSHA256 || receipt.Revision != node.Desired.Revision || receipt.PayloadSHA256 != node.Desired.PayloadSHA256 {
		t.Fatal("controller returned a different signed assignment")
	}
	require(t, safeio.Atomic(filepath.Join(f.root, "reports", "controller-"+name+"-assignment.json"), raw, 0600))
	return safeio.Hash(raw)
}

func (c *controller) denied(t *testing.T, value wire.Enrollment) (int, int, int) {
	t.Helper()
	_, assignmentErr := c.client.Assignment(t.Context(), value.NodeID, value.Credential)
	reportErr := c.client.Report(t.Context(), value.Credential, wire.Status{FleetID: "acceptance", NodeID: value.NodeID, Phase: "waiting"})
	_, enrollErr := c.client.Enroll(t.Context(), value)
	for _, err := range []error{assignmentErr, reportErr, enrollErr} {
		var failure *fleetapi.HTTPError
		if !errors.As(err, &failure) || failure.Status != 403 {
			t.Fatal("revoked identity was not rejected with HTTP 403", err)
		}
	}
	return 403, 403, 403
}
