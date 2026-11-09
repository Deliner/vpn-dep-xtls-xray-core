package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"vpn-script/internal/control/integration/evidence/diagnostics"
	"vpn-script/internal/control/integration/evidence/installation"
	"vpn-script/internal/control/integration/evidence/installation/previous"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/node/release"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
)

const installedBase = "/opt/vpn-nhop"
const installedUnit = "/etc/systemd/system/vpn-nhop.service"
const installedDropin = "/run/systemd/system/vpn-nhop.service.d"

func installedGuard(t *testing.T) {
	t.Helper()
	require(t, installation.OwnedHost())
	if !filepath.IsAbs(os.Getenv("NHOP_NODE_BINARY")) || !filepath.IsAbs(os.Getenv("NHOP_SYSTEMD_REPORT")) {
		t.Fatal("requires root in the private Lima VM and absolute binary/report paths")
	}
	if previous := os.Getenv("NHOP_PREVIOUS_NODE_BINARY"); previous != "" && !filepath.IsAbs(previous) {
		t.Fatal("previous installer path must be absolute")
	}
	for _, path := range []string{installedBase, installedUnit, installedDropin, installedUnit + ".d", "/etc/ufw/applications.d/vpn-nhop"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("refusing to change existing node state: %s", path)
		}
	}
	result, err := process.Run(context.Background(), time.Second, "id", "vpn-nhop")
	if err == nil || result.Success {
		t.Fatal("refusing to replace an existing service account")
	}
}

func installedPacketTrace(t *testing.T) {
	t.Helper()
	path := filepath.Join(filepath.Dir(os.Getenv("NHOP_SYSTEMD_REPORT")), "loopback.pcap")
	stop, err := diagnostics.CaptureLoopback(path)
	require(t, err)
	t.Cleanup(func() { require(t, stop()) })
}

func installedCleanup(t *testing.T, report *installedSuite) {
	t.Helper()
	if diagnostic, err := safeio.Read(installedBase+"/last-error.log", 65536); err == nil {
		if err := safeio.Atomic(filepath.Join(filepath.Dir(os.Getenv("NHOP_SYSTEMD_REPORT")), "last-error.log"), diagnostic, 0600); err != nil {
			t.Error(err)
		}
	}
	result, stopErr := process.Run(context.Background(), 30*time.Second, "systemctl", "disable", "--now", "vpn-nhop.service")
	if _, err := os.Lstat(installedUnit); err == nil && (stopErr != nil || !result.Success) {
		t.Errorf("cannot stop test service; preserving state: %v: %s", stopErr, result.Output)
		return
	}
	if len(report.CachedArchives) > 0 {
		if err := installation.CaptureInputs(filepath.Dir(os.Getenv("NHOP_SYSTEMD_REPORT")), report.CachedArchives, report.PreviousRuntimes); err != nil {
			t.Error(err)
		}
	}
	for _, path := range []string{installedUnit, installedBase, installedDropin} {
		if err := os.RemoveAll(path); err != nil {
			t.Error(err)
		}
	}
	installedCommand(t, "systemctl", "daemon-reload")
	_, _ = process.Run(context.Background(), time.Second, "systemctl", "reset-failed", "vpn-nhop.service")
	if result, err := process.Run(context.Background(), time.Second, "id", "vpn-nhop"); err == nil && result.Success {
		if result, err := process.Run(context.Background(), time.Second, "userdel", "vpn-nhop"); err != nil || !result.Success {
			t.Errorf("remove test service account: %v: %s", err, result.Output)
		}
	}
}

func installedCommand(t *testing.T, args ...string) []byte {
	t.Helper()
	result, err := process.Run(context.Background(), 30*time.Second, args...)
	if err != nil || !result.Success {
		t.Fatalf("%s: %v: %s", args, err, result.Output)
	}
	return result.Output
}

func installedConfig(t *testing.T, l *lab) model.NodeConfig {
	t.Helper()
	data, err := engines.Node(l.topology, "node0", l.secrets)
	require(t, err)
	for index, program := range data.Programs {
		data.Programs[index] = localProgram(program)
	}
	installedWildcards(&data)
	return installedDigest(t, data)
}

func installedInitial(t *testing.T, l *lab) model.NodeConfig {
	t.Helper()
	data := installedConfig(t, l)
	if os.Getenv("NHOP_PREVIOUS_NODE_BINARY") != "" {
		var err error
		data, err = previous.Manifest(data)
		require(t, err)
	}
	return data
}

func installedDigest(t *testing.T, data model.NodeConfig) model.NodeConfig {
	t.Helper()
	data.Generation = ""
	var err error
	data.Generation, err = safeio.Digest(data)
	require(t, err)
	require(t, manifest.Validate(data))
	return data
}

func installedApply(t *testing.T, data model.NodeConfig, expected string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "node.json")
	require(t, safeio.JSON(path, data))
	result, err := process.Run(context.Background(), 6*time.Minute, installedInstallerArgs(path)...)
	if reason, failure := installedFailures[expected]; failure {
		if err == nil || result.Success || !strings.Contains(string(result.Output), reason) {
			t.Fatalf("expected %s, got %v: %s", expected, err, result.Output)
		}
		t.Logf("expected apply failure: %s", result.Output)
		return
	}
	if err != nil || !result.Success {
		t.Fatalf("installer: %v: %s", err, result.Output)
	}
	var reply installer.Result
	require(t, json.Unmarshal(result.Output, &reply))
	if reply.Result != expected || reply.Generation != data.Generation || reply.Node != data.NodeID {
		t.Fatalf("unexpected installer reply: %+v", reply)
	}
	t.Logf("installer %s: %s", expected, data.Generation)
}

func installedChangedLogs(t *testing.T, l *lab) model.NodeConfig {
	t.Helper()
	changed := installedConfig(t, l)
	for _, program := range changed.Programs {
		key, level := "level", "debug"
		if program.Name == "xray" {
			key = "loglevel"
		}
		if current, ok := program.Config["log"].(model.Object); ok && current[key] == level {
			level = "info"
		}
		program.Config["log"] = model.Object{key: level}
	}
	return installedDigest(t, changed)
}

func installedInstallerArgs(path string) []string {
	args := []string{os.Getenv("NHOP_NODE_BINARY"), "--config", path}
	proxy := os.Getenv("NHOP_SYSTEMD_INSTALLER_PROXY")
	if proxy == "" {
		return args
	}
	// Only the installer receives these variables; systemd owns the service's environment.
	environment := []string{"/usr/bin/env", "NO_PROXY=", "no_proxy="}
	for _, name := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		environment = append(environment, name+"="+proxy)
	}
	return append(environment, args...)
}

func installedCache(t *testing.T, data model.NodeConfig) (map[string]string, int64) {
	t.Helper()
	arch, err := installer.Architecture()
	require(t, err)
	assets, err := installation.CacheAssets(data, arch, os.Getenv("NHOP_PREVIOUS_NODE_BINARY") != "")
	require(t, err)
	hashes, size, err := installation.CacheSnapshot(filepath.Join(installedBase, "cache"), assets)
	require(t, err)
	return hashes, size
}

func installedHideArchive(t *testing.T, data model.NodeConfig) func() {
	t.Helper()
	arch, err := installer.Architecture()
	require(t, err)
	asset := engines.Programs(data)[0].Runtime.Assets[arch]
	path := filepath.Join(installedBase, "cache", asset.SHA256+".archive")
	directory, err := os.MkdirTemp(installedBase, ".cache-check-")
	require(t, err)
	t.Cleanup(func() { require(t, os.RemoveAll(directory)) })
	backup := filepath.Join(directory, "archive")
	require(t, os.Rename(path, backup))
	return func() { require(t, os.Rename(backup, path)) }
}

func installedUnchanged(t *testing.T, data model.NodeConfig) {
	t.Helper()
	pid := installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")
	installedApply(t, data, "unchanged")
	if !bytes.Equal(pid, installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")) {
		t.Fatal("idempotent installation restarted the service")
	}
}

func installedPath(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(installedBase + "/current")
	require(t, err)
	return directory
}

func installedCheckpoint(t *testing.T, data model.NodeConfig) installedRevision {
	t.Helper()
	point := installedRevision{Directory: installedPath(t)}
	require(t, safeio.LoadJSON(installedBase+"/state.json", &point.State))
	var err error
	point.Unit, err = os.ReadFile(installedUnit)
	require(t, err)
	id := point.State.ReleaseID
	if id == "" {
		id = point.State.Generation
	}
	if point.State.Generation != data.Generation || filepath.Base(point.Directory) != id {
		t.Fatal("committed configuration and release directory disagree")
	}
	hashes, err := release.Inventory(point.Directory, data)
	require(t, err)
	if !reflect.DeepEqual(hashes, point.State.Files) {
		t.Fatal("committed release inventory does not match the installed files")
	}
	return point
}

func installedRejectNewSupervisor(t *testing.T, previous installedRevision) func() {
	t.Helper()
	require(t, os.Mkdir(installedDropin, 0755))
	reset := func() {
		require(t, os.RemoveAll(installedDropin))
		installedCommand(t, "systemctl", "daemon-reload")
	}
	t.Cleanup(reset)
	// Allow only the previous executable inode. This rejects activation after
	// current switches, but lets the real rollback restart the previous service.
	unit := "[Service]\nExecStartPre=/usr/bin/test " + installedBase + "/current/nhop-node -ef " + previous.Directory + "/nhop-node\n"
	require(t, safeio.Atomic(filepath.Join(installedDropin, "acceptance.conf"), []byte(unit), 0644))
	installedCommand(t, "systemctl", "daemon-reload")
	return reset
}

func saveInstalledReport(t *testing.T, report *installedSuite) {
	t.Helper()
	report.Status = "passed"
	if t.Failed() {
		report.Status = "failed"
	}
	path := os.Getenv("NHOP_SYSTEMD_REPORT")
	if !filepath.IsAbs(path) || !strings.HasSuffix(path, "/installed.json") {
		t.Error("invalid installed report path")
		return
	}
	if err := safeio.JSON(path, report); err != nil {
		t.Error(err)
	}
}
