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
	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/release"
	"vpn-script/internal/platform/safeio"
)

func TestInstalledNodeLifecycle(t *testing.T) {
	if os.Getenv("NHOP_SYSTEMD_ACCEPTANCE") != "private-lima" {
		t.Skip("explicit private Linux VM acceptance only; use just lab-installer")
	}
	report := installedSuite{SchemaVersion: 1}
	t.Cleanup(func() { saveInstalledReport(t, &report) })
	installedGuard(t)
	installedPacketTrace(t)
	t.Cleanup(func() { installedCleanup(t) })
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
	initial := installedConfig(t, l)
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
	installedUpgrade(t, l, initial, binary, &report)
	installedAccessControls(t, l, initial, &report)
	installedUpdateOffline(t, l, initial, &report)
	require(t, evidence.Verify(report.Runtimes))
}

func installedBindings(t *testing.T, data model.NodeConfig) (int, bool) {
	t.Helper()
	ports := map[string]int{}
	for _, listener := range data.Listeners {
		ports[listener.Tag+"/"+listener.Network] = listener.Port
	}
	tcp := ports["node0."+model.Reality+"/tcp"]
	if tcp == 0 || ports["node0.tuic/udp"] != tcp {
		t.Fatal("installed manifest lost its shared TCP/UDP listener pair")
	}
	return tcp, ports["node0.hysteria2/udp"] == 22
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
	installedUpdate(t, l, initial, report)
	hashes, size := installedCache(t, initial)
	report.CacheWarmRequests = requests.Load()
	if report.CacheWarmRequests != 0 || size != report.CachedBytes || !reflect.DeepEqual(hashes, report.CachedArchives) {
		t.Fatal("warm cache update accessed the network or changed verified archive contents")
	}
	report.CacheOffline = true
	t.Logf("cache: %d verified bytes; cold probe %d requests, warm lifecycle %d requests", size, report.CacheProbeRequests, report.CacheWarmRequests)
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

func installedOldPorts(t *testing.T, initial, updated model.NodeConfig) {
	t.Helper()
	current := map[string]bool{}
	for _, listener := range updated.Listeners {
		current[listener.Network+"/"+strconv.Itoa(listener.Port)] = true
	}
	for _, listener := range initial.Listeners {
		if current[listener.Network+"/"+strconv.Itoa(listener.Port)] {
			continue
		}
		address := net.JoinHostPort("127.0.0.1", strconv.Itoa(listener.Port))
		if listener.Network == "tcp" {
			bound, err := net.Listen("tcp4", address)
			require(t, err)
			require(t, bound.Close())
		} else {
			bound, err := net.ListenPacket("udp4", address)
			require(t, err)
			require(t, bound.Close())
		}
	}
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

func installedUpgrade(t *testing.T, l *lab, data model.NodeConfig, binary string, report *installedSuite) {
	t.Helper()
	if os.Getenv("NHOP_PREVIOUS_NODE_BINARY") == "" {
		t.Log("installer upgrade not exercised: provide a previous Linux installer and its SHA-256")
		return
	}
	before := installedCheckpoint(t, data)
	report.PreviousSHA = before.State.Files["nhop-node"]
	var err error
	report.InstallerSHA, err = safeio.FileHash(binary)
	require(t, err)
	if report.PreviousSHA == report.InstallerSHA {
		t.Fatal("upgrade requires different previous/current installer bytes")
	}
	t.Setenv("NHOP_NODE_BINARY", binary)
	installedUpgradeFailure(t, l, data, before, report)
	installedApply(t, data, "applied")
	after := installedCheckpoint(t, data)
	if before.Directory == after.Directory || after.State.PreviousReleaseID != filepath.Base(before.Directory) || after.State.PreviousGeneration != data.Generation || after.State.Files["nhop-node"] != report.InstallerSHA {
		t.Fatal("same-configuration upgrade lost release history or kept the old supervisor")
	}
	retained, err := release.Inventory(before.Directory, data)
	require(t, err)
	if !reflect.DeepEqual(retained, before.State.Files) {
		t.Fatal("installer upgrade modified the retained previous release")
	}
	report.Upgraded, report.Migrated = true, before.State.ReleaseID == ""
	report.ReleaseIDs = append(report.ReleaseIDs, after.State.ReleaseID)
	installedTraffic(t, l, "installer-upgraded", report)
	installedUnchanged(t, data)
	installedUnitRepair(t, l, data, before.Unit, after, report)
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

func installedUpgradeFailure(t *testing.T, l *lab, data model.NodeConfig, previous installedRevision, report *installedSuite) {
	t.Helper()
	pid := installedCommand(t, "systemctl", "show", "-p", "MainPID", "--value", "vpn-nhop.service")
	before, err := os.ReadFile(installedBase + "/state.json")
	require(t, err)
	reset := installedRejectNewSupervisor(t, previous)
	installedApply(t, data, "start-failed")
	after := installedCheckpoint(t, data)
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
