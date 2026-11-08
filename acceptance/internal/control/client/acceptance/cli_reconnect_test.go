package acceptance

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	clientcontrol "vpn-script/internal/control/client/controller"
)

func reconnectPublic(t *testing.T, probe *controller, initial response, done <-chan error, started clientcontrol.Diagnostic, role string, dns *dnsOrigins) {
	t.Helper()
	canary := directCanary(t)
	watch := startLeakWatch(t)
	refuseRunningReconnect(t, started)
	for cycle := range 2 {
		pid := started.Status.NativePID
		if role == "frontend" {
			pid = started.Status.FrontendPID
		}
		if syscall.Kill(pid, syscall.SIGKILL) != nil {
			t.Fatal("cannot stop owned reconnect engine")
		}
		waitFailure(t)
		checkTraffic(t, probe, false)
		checkSystemDNS(t, probe, dns, "blocked")
		if cycle == 0 {
			refuseReconnectChanges(t, probe, started)
			for _, field := range []int{6, 2} {
				refuseReconnectIdentity(t, started, field)
			}
		}
		started = reconnectAndVerify(t, probe, started, dns)
	}
	watch.stop(t)
	if watch.attempts < 20 || canary.Load() != 0 {
		t.Fatalf("continuous direct leak control: attempts=%d delivered=%d", watch.attempts, canary.Load())
	}
	t.Logf("reconnect direct UDP: attempts=%d delivered=0; same firewall lease retained", watch.attempts)
	finishPublic(t, probe, initial, done, "graceful", dns)
	if result := probe.call(t, "probe-direct"); result.Error != "" || canary.Load() == 0 {
		t.Fatalf("direct canary was not demonstrably reachable after stop: %+v", result)
	}
}

func refuseRunningReconnect(t *testing.T, started clientcontrol.Diagnostic) {
	t.Helper()
	if _, err := cliCommand(t, "reconnect").Output(); err == nil {
		t.Fatal("running session accepted reconnect")
	}
	current := cliCall(t, "diagnostics")
	if current.Error != "" || current.Status != started.Status {
		t.Fatal("rejected reconnect changed healthy session status")
	}
}

func refuseReconnectIdentity(t *testing.T, started clientcontrol.Diagnostic, field int) {
	t.Helper()
	checkIsolation(t)
	original, err := os.ReadFile("/etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	changed := changedIdentity(t, original, field)
	if err := os.WriteFile("/etc/passwd", changed, 0644); err != nil {
		t.Fatal(err)
	}
	_, commandErr := cliCommand(t, "reconnect").Output()
	observed, readErr := os.ReadFile("/etc/passwd")
	restoreErr := os.WriteFile("/etc/passwd", original, 0644)
	if commandErr == nil || readErr != nil || restoreErr != nil || !bytes.Equal(observed, changed) {
		t.Fatal("unsafe NSS identity was accepted, repaired or could not be restored", commandErr, readErr, restoreErr)
	}
	current := cliCall(t, "diagnostics")
	if current.Status.Phase != "failed-closed" || current.Status.Guard != started.Status.Guard {
		t.Fatal("rejected NSS identity changed phase or guard")
	}
	if field == 2 && !strings.Contains(current.Error, "client NSS identity changed") {
		t.Fatal("valid reassigned NSS identity was not rejected by numeric ownership comparison", current.Error)
	}
}

func changedIdentity(t *testing.T, original []byte, field int) []byte {
	t.Helper()
	lines := strings.Split(string(original), "\n")
	for i, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) == 7 && fields[0] == "nhop-client" {
			fields[field] = "/bin/sh"
			if field == 2 {
				fields[field] = "2147483000"
			}
			lines[i] = strings.Join(fields, ":")
		}
	}
	changed := []byte(strings.Join(lines, "\n"))
	if bytes.Equal(original, changed) {
		t.Fatal("private NSS test did not alter the service account")
	}
	return changed
}

func reconnectAndVerify(t *testing.T, probe *controller, previous clientcontrol.Diagnostic, dns *dnsOrigins) clientcontrol.Diagnostic {
	t.Helper()
	logs := map[string][]byte{}
	for _, name := range []string{"native.log", "frontend.log"} {
		data, err := os.ReadFile(filepath.Join(previous.Directory, name))
		if err != nil {
			t.Fatal(err)
		}
		logs[name] = data
	}
	result := cliCall(t, "reconnect")
	if result.Error != "" || result.Status.Phase != "running" || result.Directory != previous.Directory || result.Status.Guard != previous.Status.Guard {
		t.Fatalf("reconnect changed guard ownership or did not run: %+v", result)
	}
	if result.Status.NativePID <= 1 || result.Status.FrontendPID <= 1 || result.Status.NativePID == previous.Status.NativePID || result.Status.FrontendPID == previous.Status.FrontendPID {
		t.Fatal("reconnect reused stopped processes")
	}
	for name, before := range logs {
		after, err := os.ReadFile(filepath.Join(previous.Directory, name))
		if err != nil || !bytes.HasPrefix(after, before) {
			t.Fatal("reconnect lost prior process logs", err)
		}
	}
	activeTraffic(t, probe)
	checkSystemDNS(t, probe, dns, "tunnel")
	return result
}

func refuseReconnectChanges(t *testing.T, probe *controller, started clientcontrol.Diagnostic) {
	t.Helper()
	for _, name := range []string{"native", "frontend.json"} {
		path := filepath.Join(started.Directory, name)
		file, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			t.Fatal(err)
		}
		var first [1]byte
		_, readErr := file.ReadAt(first[:], 0)
		_, writeErr := file.WriteAt([]byte{first[0] ^ 1}, 0)
		if readErr != nil || writeErr != nil {
			file.Close()
			t.Fatal(readErr, writeErr)
		}
		before := probe.call(t, "routes").Routes
		_, commandErr := cliCommand(t, "reconnect").Output()
		_, restoreErr := file.WriteAt(first[:], 0)
		closeErr := file.Close()
		if commandErr == nil || restoreErr != nil || closeErr != nil {
			t.Fatal("changed snapshot was accepted or could not be restored", commandErr, restoreErr, closeErr)
		}
		current := cliCall(t, "diagnostics")
		if current.Status.Phase != "failed-closed" || current.Status.Guard != started.Status.Guard || probe.call(t, "routes").Routes != before {
			t.Fatal("rejected reconnect changed routes, guard or phase")
		}
		checkTraffic(t, probe, false)
	}
}
