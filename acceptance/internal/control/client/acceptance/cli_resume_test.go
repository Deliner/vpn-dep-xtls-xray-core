package acceptance

import (
	"context"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	clientcontrol "vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

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
