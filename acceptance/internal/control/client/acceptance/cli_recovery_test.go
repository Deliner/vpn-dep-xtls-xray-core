package acceptance

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	clientcontrol "vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
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

func resumePublic(t *testing.T, probe *controller, initial response, command *exec.Cmd, done <-chan error, started clientcontrol.Diagnostic, role string, dns *dnsOrigins) {
	t.Helper()
	canary, watch := directCanary(t), startLeakWatch(t)
	sentinel, exited := resumeSentinel(t)
	cliRejects(t, "resume")
	for range 2 {
		interruptController(t, command, done, role)
		checkTraffic(t, probe, false)
		checkSystemDNS(t, probe, dns, "blocked")
		refuseChangedLease(t, probe, started.Directory)
		refuseResumeRequest(t, probe, started.Directory)
		replaceRecordedPIDs(t, started.Directory, sentinel)
		previous := started
		command, done, started = cliForeground(t, "resume")
		if started.Error != "" || started.Status.Phase != "running" || started.Directory != previous.Directory || started.Status.Guard != previous.Status.Guard {
			t.Fatalf("resume did not retain the protected session: %+v", started)
		}
		assertSentinelAlive(t, exited)
		activeTraffic(t, probe)
		checkSystemDNS(t, probe, dns, "tunnel")
	}
	watch.stop(t)
	if watch.attempts < 20 || canary.Load() != 0 {
		t.Fatalf("resume direct leak control: attempts=%d delivered=%d", watch.attempts, canary.Load())
	}
	t.Logf("resume direct UDP: attempts=%d delivered=0; same firewall lease retained", watch.attempts)
	finishPublic(t, probe, initial, done, "graceful", dns)
	cliRejects(t, "resume")
	assertSentinelAlive(t, exited)
	if result := probe.call(t, "probe-direct"); result.Error != "" || canary.Load() == 0 {
		t.Fatal("resume direct canary did not become reachable after stop", result)
	}
}

func interruptController(t *testing.T, command *exec.Cmd, done <-chan error, role string) {
	t.Helper()
	signal := syscall.SIGTERM
	if role == "kill" {
		signal = syscall.SIGKILL
	}
	if err := command.Process.Signal(signal); err != nil {
		t.Fatal(err)
	}
	cliWait(t, done, false)
	if cliCall(t, "diagnostics").Controller != "unavailable" {
		t.Fatal("interrupted controller was reported available")
	}
}

func refuseResumeRequest(t *testing.T, probe *controller, directory string) {
	t.Helper()
	path := filepath.Join(directory, "request.json")
	var original session.Request
	if err := safeio.LoadJSON(path, &original); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := safeio.JSON(path, original); err != nil {
			t.Error(err)
		}
	}()
	changed := original
	changed.Directory = filepath.Join(filepath.Dir(directory), "unowned")
	if err := safeio.JSON(path, changed); err != nil {
		t.Fatal(err)
	}
	before := probe.call(t, "routes").Routes
	cliRejects(t, "resume")
	if probe.call(t, "routes").Routes != before {
		t.Fatal("refused journal identity changed routes")
	}
	checkTraffic(t, probe, false)
}

func replaceRecordedPIDs(t *testing.T, directory string, pid int) {
	t.Helper()
	path := filepath.Join(directory, "status.json")
	var status session.Status
	if err := safeio.LoadJSON(path, &status); err != nil {
		t.Fatal(err)
	}
	status.NativePID, status.FrontendPID = pid, pid
	if err := safeio.JSON(path, status); err != nil {
		t.Fatal(err)
	}
}

func resumeSentinel(t *testing.T) (int, <-chan error) {
	t.Helper()
	command := process.Command(context.Background(), "sleep", "90")
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- command.Wait(); close(exited) }()
	t.Cleanup(func() { process.Stop(command); <-exited })
	return command.Process.Pid, exited
}

func assertSentinelAlive(t *testing.T, exited <-chan error) {
	t.Helper()
	select {
	case err := <-exited:
		t.Fatal("recovery signalled an unrelated recorded PID", err)
	default:
	}
}
