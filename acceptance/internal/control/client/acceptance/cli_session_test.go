package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	clientcontrol "vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/platform/safeio"
)

func failPublic(t *testing.T, probe *controller, command *exec.Cmd, done <-chan error, started clientcontrol.Diagnostic, failure string) {
	t.Helper()
	if failure == "native" {
		if syscall.Kill(started.Status.NativePID, syscall.SIGKILL) != nil {
			t.Fatal("cannot stop native test engine")
		}
		waitFailure(t)
	} else if failure != "graceful" {
		signal := syscall.SIGTERM
		if failure == "controller-kill" {
			signal = syscall.SIGKILL
		}
		if command.Process.Signal(signal) != nil {
			t.Fatal("cannot stop owned controller")
		}
		cliWait(t, done, false)
		if cliCall(t, "diagnostics").Controller != "unavailable" {
			t.Fatal("dead controller was reported active")
		}
		refuseChangedLease(t, probe, started.Directory)
	}
}

func finishPublic(t *testing.T, probe *controller, initial response, done <-chan error, failure string, dns *dnsOrigins) {
	t.Helper()
	if failure != "graceful" {
		checkTraffic(t, probe, false)
		checkSystemDNS(t, probe, dns, "blocked")
	}
	if result := cliCall(t, "stop"); result.Error != "" || result.Status.Phase != "stopped" {
		t.Fatalf("public stop: %+v", result)
	}
	if failure == "graceful" || failure == "native" {
		cliWait(t, done, true)
	}
	checkTraffic(t, probe, true)
	checkSystemDNS(t, probe, dns, "direct")
	if probe.call(t, "dns").DNS != initial.DNS || probe.call(t, "routes").Routes != initial.Routes {
		t.Fatal("public stop/recovery did not restore DNS and policy rules")
	}
}

func cliInputs(t *testing.T, request session.Request) (string, string) {
	t.Helper()
	profile := filepath.Join(filepath.Dir(request.Directory), "profile.json")
	runtimes := filepath.Join(filepath.Dir(request.Directory), "runtimes.json")
	if err := safeio.NewJSON(profile, request.Profile); err != nil {
		t.Fatal(err)
	}
	value := clientcontrol.Runtimes{SchemaVersion: 1, Xray: request.Native, SingBox: request.Frontend, Cronet: request.Cronet}
	if err := safeio.NewJSON(runtimes, value); err != nil {
		t.Fatal(err)
	}
	return profile, runtimes
}

func verifyIdentity(t *testing.T, value clientcontrol.Diagnostic) {
	t.Helper()
	var request session.Request
	if err := safeio.LoadJSON(filepath.Join(value.Directory, "request.json"), &request); err != nil {
		t.Fatal(err)
	}
	if request.Network.EngineUID < 100 || request.EngineGID < 100 || request.Network.EngineUID == 20000 {
		t.Fatal("public client did not resolve its own dedicated identity")
	}
	if err := safeio.CheckAccess(host.Socket, os.ModeSocket|0600, 0, 0); err != nil {
		t.Fatal(err)
	}
}

func waitFailure(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cliCall(t, "diagnostics").Status.Phase == "failed-closed" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("controller did not observe engine failure")
}

func activeTraffic(t *testing.T, probe *controller) {
	t.Helper()
	for _, operation := range []string{"probe", "probe-udp", "probe6"} {
		allowed := operation != "probe6" || os.Getenv("NHOP_CLIENT_IPV6") == "tunnel"
		result := probe.call(t, operation)
		if (result.Error == "") != allowed {
			t.Fatalf("active %s policy mismatch: allowed=%v response=%+v", operation, allowed, result)
		}
	}
}

func refuseDuplicate(t *testing.T, profile, runtimes string, started clientcontrol.Diagnostic) {
	t.Helper()
	data, err := cliCommand(t, "start", "--profile", profile, "--runtimes", runtimes).Output()
	if err == nil {
		t.Fatalf("second controller acquired active state: %s", data)
	}
	actual := cliCall(t, "diagnostics")
	if actual.Controller != "active" || actual.Status.NativePID != started.Status.NativePID || actual.Directory != started.Directory {
		t.Fatal("second controller changed the active session")
	}
	verifyPreviousSnapshots(t, started.Directory)
}

func verifyPreviousSnapshots(t *testing.T, active string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(host.Directory, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		directory := filepath.Join(host.Directory, "sessions", entry.Name())
		if directory == active {
			continue
		}
		for _, name := range []string{"native", "frontend", "libcronet.so"} {
			if _, err := os.Lstat(filepath.Join(directory, name)); !os.IsNotExist(err) {
				t.Fatal("stopped runtime snapshot was not pruned", name, err)
			}
		}
		if err := safeio.CheckAccess(filepath.Join(directory, "request.json"), 0600, 0, 0); err != nil {
			t.Fatal("cleanup lost the private session journal", err)
		}
	}
}

func refuseChangedLease(t *testing.T, probe *controller, directory string) {
	t.Helper()
	path := filepath.Join(directory, "status.json")
	var original session.Status
	if err := safeio.LoadJSON(path, &original); err != nil {
		t.Fatal(err)
	}
	defer func() { safeio.JSON(path, original) }()
	changed := original
	changed.Guard.Token = "nhop:" + strings.Repeat("f", 32)
	if err := safeio.JSON(path, changed); err != nil {
		t.Fatal(err)
	}
	before := probe.call(t, "routes").Routes
	for _, command := range []string{"stop", "resume"} {
		cliRejects(t, command)
	}
	if probe.call(t, "routes").Routes != before {
		t.Fatal("rejected recovery mutated routing rules")
	}
	checkTraffic(t, probe, false)
}

func refuseBadRuntime(t *testing.T, probe *controller, profile, runtimes string) {
	t.Helper()
	var inventory clientcontrol.Runtimes
	if err := safeio.LoadJSON(runtimes, &inventory); err != nil {
		t.Fatal(err)
	}
	inventory.Xray.SHA256 = strings.Repeat("0", 64)
	path := filepath.Join(filepath.Dir(profile), "tampered-runtimes.json")
	if err := safeio.NewJSON(path, inventory); err != nil {
		t.Fatal(err)
	}
	before := sessionNames(t)
	journal, err := os.ReadFile(filepath.Join(host.Directory, "current.json"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	routes, dns := probe.call(t, "routes").Routes, probe.call(t, "dns").DNS
	if data, err := cliCommand(t, "start", "--profile", profile, "--runtimes", path).Output(); err == nil {
		t.Fatalf("changed runtime was executed: %s", data)
	}
	if sessionNames(t) != before || probe.call(t, "routes").Routes != routes || probe.call(t, "dns").DNS != dns {
		t.Fatal("failed runtime snapshot changed session inventory or host networking")
	}
	after, err := os.ReadFile(filepath.Join(host.Directory, "current.json"))
	if err != nil && !os.IsNotExist(err) || string(after) != string(journal) {
		t.Fatal("unstarted session lost the prior journal")
	}
	checkTraffic(t, probe, true)
}

func sessionNames(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(host.Directory, "sessions"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return strings.Join(names, ",")
}
