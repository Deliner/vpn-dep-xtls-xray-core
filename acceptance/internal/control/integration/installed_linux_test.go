package integration

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/control/integration/evidence/installation"
	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

func TestInstalledNodeLifecycle(t *testing.T) {
	if mode := os.Getenv("NHOP_SYSTEMD_ACCEPTANCE"); mode != "private-lima" && mode != "owned-native-ci" {
		t.Skip("explicit private Linux VM acceptance only; use just lab-installer")
	}
	report := installedSuite{SchemaVersion: 1}
	t.Cleanup(func() { saveInstalledReport(t, &report) })
	installedGuard(t)
	installedPacketTrace(t)
	t.Cleanup(func() { installedCleanup(t, &report) })
	t.Setenv("NHOP_SYSTEMD_INSTALLER_PROXY", "")
	binary := os.Getenv("NHOP_NODE_BINARY")
	if previous := os.Getenv("NHOP_PREVIOUS_NODE_BINARY"); previous != "" {
		t.Setenv("NHOP_NODE_BINARY", previous)
	}
	t.Setenv("XRAY_BINARY", installedBase+"/current/xray")
	t.Setenv("SINGBOX_BINARY", installedBase+"/current/sing-box")
	l := matrixPrepare(t, 1)
	installedPublicPorts(t, l)
	report.Profiles = dag.Keys(l.topology.Routes)
	initial := installedInitial(t, l)
	installedApply(t, initial, "applied")
	report.Generations = append(report.Generations, initial.Generation)
	report.ReleaseIDs = append(report.ReleaseIDs, filepath.Base(installedPath(t)))
	var err error
	report.Runtimes, err = evidence.Capture(context.Background(), l.runtimes.Inputs)
	require(t, err)
	installedTraffic(t, l, "initial", &report)
	port, udpSSH := installedBindings(t, initial)
	if port != 443 || !udpSSH {
		t.Fatal("installed fixture lost its 443 TCP/UDP or SSH-numbered UDP listener")
	}
	report.SharedPorts, report.UDPWithSSH = []int{port}, udpSSH
	report.WildcardListeners = installedWildcardTags(t, initial)
	installedUnchanged(t, initial)
	report.UnchangedPID = true
	initial = installedUpgrade(t, l, initial, binary, &report)
	installedAccessControls(t, l, initial, &report)
	installedUpdateOffline(t, l, initial, &report)
	require(t, installation.VerifyRuntimeHistory(report.Runtimes, report.PreviousRuntimes))
}

func installedUpdateOffline(t *testing.T, l *lab, initial model.NodeConfig, report *installedSuite) {
	t.Helper()
	report.CachedArchives, report.CachedBytes = installedCache(t, initial)
	proxy, requests := rejectingProxy(t)
	t.Setenv("NHOP_SYSTEMD_INSTALLER_PROXY", proxy)
	installedCacheMiss(t, l, initial)
	report.CacheProbeRequests = requests.Swap(0)
	if report.CacheProbeRequests == 0 {
		t.Fatal("cold cache probe did not reach the rejecting proxy")
	}
	report.CacheMissBlocked = true
	installedInterrupted(t, l, initial, report)
	installedUpdate(t, l, initial, report)
	hashes, size := installedCache(t, initial)
	report.CacheWarmRequests = requests.Load()
	if report.CacheWarmRequests != 0 || size != report.CachedBytes || !reflect.DeepEqual(hashes, report.CachedArchives) {
		t.Fatal("warm cache update accessed the network or changed verified archive contents")
	}
	report.CacheOffline = true
	t.Logf("cache: %d verified bytes; cold probe %d requests, warm lifecycle %d requests", size, report.CacheProbeRequests, report.CacheWarmRequests)
}

func installedInterrupted(t *testing.T, l *lab, data model.NodeConfig, report *installedSuite) {
	t.Helper()
	before := installedCheckpoint(t, data)
	path := filepath.Join(t.TempDir(), "interrupted-node.json")
	require(t, safeio.JSON(path, installedChangedLogs(t, l)))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	require(t, installation.Gate(ctx))
	reset := func() { require(t, installation.Ungate(context.Background())) }
	defer reset()
	proof, err := installation.KillInstaller(ctx, installedInstallerArgs(path), before.Directory, filepath.Dir(os.Getenv("NHOP_SYSTEMD_REPORT")))
	require(t, err)
	reset()
	installedApply(t, data, "unchanged")
	if after := installedCheckpoint(t, data); !reflect.DeepEqual(before, after) {
		t.Fatal("SIGKILL recovery did not restore the previous release, unit and state")
	}
	if _, err := os.Lstat(installedBase + "/transaction.json"); !os.IsNotExist(err) {
		t.Fatal("successful interrupted installation recovery retained its journal")
	}
	proof.Restored = true
	report.Interruption = &proof
	installedTraffic(t, l, "interrupted-restored", report)
	installedUnchanged(t, data)
}

func installedCacheMiss(t *testing.T, l *lab, initial model.NodeConfig) {
	t.Helper()
	before := installedCheckpoint(t, initial)
	pid := installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")
	state, err := os.ReadFile(installedBase + "/state.json")
	require(t, err)
	restore := installedHideArchive(t, initial)
	defer restore()
	changed := installedChangedLogs(t, l)
	if changed.Generation == initial.Generation {
		t.Fatal("cold cache probe requires a different configuration")
	}
	installedApply(t, changed, "download-failed")
	after := installedCheckpoint(t, initial)
	restored, err := os.ReadFile(installedBase + "/state.json")
	require(t, err)
	if !reflect.DeepEqual(before, after) || !bytes.Equal(state, restored) || !bytes.Equal(pid, installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")) {
		t.Fatal("failed cold cache preparation changed the committed service")
	}
}

func installedUpdate(t *testing.T, l *lab, initial model.NodeConfig, report *installedSuite) {
	t.Helper()
	updated := installedNextPorts(t, l, report)
	installedApply(t, updated, "applied")
	port, _ := installedBindings(t, updated)
	report.SharedPorts = append(report.SharedPorts, port)
	report.Generations = append(report.Generations, updated.Generation)
	report.ReleaseIDs = append(report.ReleaseIDs, filepath.Base(installedPath(t)))
	installedOldPorts(t, initial, updated)
	report.PortsReleased = true
	installedTraffic(t, l, "updated", report)
	installedRollback(t, l, updated, report)
	installedCommand(t, "systemctl", "stop", "vpn-nhop.service")
	for property, expected := range map[string]string{"Result": "success", "ExecMainStatus": "0", "SubState": "dead"} {
		actual := installedCommand(t, "systemctl", "show", "-p", property, "--value", "vpn-nhop.service")
		if strings.TrimSpace(string(actual)) != expected {
			t.Fatalf("ordinary service stop reported %s=%s instead of %s", property, actual, expected)
		}
	}
	report.CleanStop = true
	installedStoppedIdentity(t, updated, report)
	installedApply(t, updated, "unchanged")
	installedTraffic(t, l, "recovered", report)
	report.Recovered = true
	checkTCPListener(t, "127.0.0.1:22")
	installedCheckAuthority(t, report)
	checkTCPListener(t, report.DisjointForeignSocket)
	report.DisjointForeignPreserved = true
}

func installedNextPorts(t *testing.T, l *lab, report *installedSuite) model.NodeConfig {
	t.Helper()
	shared := l.port(t)
	for name, endpoint := range l.topology.Services {
		endpoint.Service.Port = l.matrixPort(t, endpoint.Service.Protocol, shared)
		l.topology.Services[name] = endpoint
		l.topology.Nodes[endpoint.Node].Services[endpoint.ID].Port = endpoint.Service.Port
	}
	foreign, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.2", strconv.Itoa(shared)))
	require(t, err)
	t.Cleanup(func() { require(t, foreign.Close()) })
	report.DisjointForeignSocket = foreign.Addr().String()
	return installedConfig(t, l)
}

func installedRollback(t *testing.T, l *lab, updated model.NodeConfig, report *installedSuite) {
	t.Helper()
	previous := installedPath(t)
	pid := installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")
	state, err := os.ReadFile(installedBase + "/state.json")
	require(t, err)
	bad := installedConfig(t, l)
	directory, err := os.MkdirTemp("/root", "nhop-systemd-")
	require(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	for _, program := range bad.Programs {
		if program.Name == "sing-box" {
			program.Config["log"] = model.Object{"level": "info", "output": filepath.Join(directory, "denied.log")}
		}
	}
	installedApply(t, installedDigest(t, bad), "failed")
	current, err := filepath.EvalSymlinks(installedBase + "/current")
	require(t, err)
	restored, err := os.ReadFile(installedBase + "/state.json")
	require(t, err)
	if current != previous || !bytes.Equal(state, restored) {
		t.Fatal("failed activation did not restore committed generation and state")
	}
	if bytes.Equal(pid, installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")) {
		t.Fatal("failed generation never replaced the running service")
	}
	installedTraffic(t, l, "rolled-back", report)
	report.RolledBack = true
}

func installedUpgrade(t *testing.T, l *lab, data model.NodeConfig, binary string, report *installedSuite) model.NodeConfig {
	t.Helper()
	if os.Getenv("NHOP_PREVIOUS_NODE_BINARY") == "" {
		t.Log("installer upgrade not exercised: provide a previous Linux installer and its SHA-256")
		return data
	}
	before := installedCheckpoint(t, data)
	report.PreviousSHA = before.State.Files["nhop-node"]
	var err error
	report.InstallerSHA, err = installation.UpgradeSupervisor(report.PreviousSHA, binary)
	require(t, err)
	t.Setenv("NHOP_NODE_BINARY", binary)
	current := installedConfig(t, l)
	installedUpgradeFailure(t, l, current, data, before, report)
	require(t, installation.RetainRuntimes(report.Runtimes, before.Directory))
	report.PreviousRuntimes = report.Runtimes
	installedApply(t, current, "applied")
	after := installedCheckpoint(t, current)
	require(t, installation.VerifyUpgrade(before, after, data, report.InstallerSHA))
	report.Runtimes, err = evidence.Capture(context.Background(), l.runtimes.Inputs)
	require(t, err)
	require(t, installation.RuntimeTransition(report.PreviousRuntimes, report.Runtimes))
	report.RuntimeUpgraded = true
	report.Upgraded, report.Migrated = true, before.State.ReleaseID == ""
	report.Generations = append(report.Generations, current.Generation)
	report.ReleaseIDs = append(report.ReleaseIDs, after.State.ReleaseID)
	installedTraffic(t, l, "installer-upgraded", report)
	installedUnchanged(t, current)
	installedUnitRepair(t, l, current, before.Unit, after, report)
	return current
}

func installedUnitRepair(t *testing.T, l *lab, data model.NodeConfig, previous []byte, current installedRevision, report *installedSuite) {
	t.Helper()
	if bytes.Equal(previous, current.Unit) {
		t.Log("previous installer used the same service template; unit repair not exercised")
		return
	}
	require(t, safeio.Atomic(installedUnit, previous, 0644))
	installedCommand(t, "systemctl", "daemon-reload")
	installedApply(t, data, "applied")
	after := installedCheckpoint(t, data)
	if after.Directory != current.Directory || !bytes.Equal(after.Unit, current.Unit) || !reflect.DeepEqual(after.State, current.State) {
		t.Fatal("unit repair changed release history or failed to restore the current template")
	}
	report.UnitRepaired = true
	installedTraffic(t, l, "unit-repaired", report)
	installedUnchanged(t, data)
}

func installedUpgradeFailure(t *testing.T, l *lab, data, prior model.NodeConfig, previous installedRevision, report *installedSuite) {
	t.Helper()
	pid := installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")
	before, err := os.ReadFile(installedBase + "/state.json")
	require(t, err)
	reset := installedRejectNewSupervisor(t, previous)
	installedApply(t, data, "start-failed")
	after := installedCheckpoint(t, prior)
	state, err := os.ReadFile(installedBase + "/state.json")
	require(t, err)
	if after.Directory != previous.Directory || !bytes.Equal(before, state) || !bytes.Equal(after.Unit, previous.Unit) {
		t.Fatal("failed supervisor upgrade did not restore the previous unit, state and release")
	}
	if bytes.Equal(pid, installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")) {
		t.Fatal("supervisor upgrade failed before actually replacing the running service")
	}
	entries, err := os.ReadDir(filepath.Dir(previous.Directory))
	require(t, err)
	if len(entries) != 1 || entries[0].Name() != filepath.Base(previous.Directory) {
		t.Fatal("failed supervisor upgrade retained a newly prepared release")
	}
	installedTraffic(t, l, "installer-rolled-back", report)
	reset()
	report.UpgradeRollback = true
}
