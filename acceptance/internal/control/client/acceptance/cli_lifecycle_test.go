package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	clientcontrol "vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/platform/safeio"
)

func TestKernelPublicClientAndControllerRecovery(t *testing.T) {
	if os.Getenv("NHOP_CLIENT_PUBLIC_TEST") != "private-netns" {
		t.Skip("requires isolated Linux namespaces, private NSS and verified engines")
	}
	checkIsolation(t)
	t.Logf("client policy: IPv6=%s", os.Getenv("NHOP_CLIENT_IPV6"))
	origin(t, "198.18.0.2:18080")
	origin(t, "[2001:db8:feed::2]:18080")
	udpOrigin(t)
	dns := startDNSOrigins(t)
	for _, failure := range []string{"graceful", "native", "controller-term", "controller-kill"} {
		t.Run(failure, func(t *testing.T) { publicLifecycle(t, failure, dns) })
	}
}

func publicLifecycle(t *testing.T, failure string, dns *dnsOrigins) {
	t.Helper()
	request := fixture(t)
	probe, initial := newController(t, request)
	checkSystemDNS(t, probe, dns, "direct")
	profile, runtimes := cliInputs(t, request)
	refuseBadRuntime(t, probe, profile, runtimes)
	command, done, started := cliStart(t, profile, runtimes)
	if started.Error != "" || started.Status.Phase != "running" {
		t.Fatalf("public start: %+v", started)
	}
	activeTraffic(t, probe)
	checkSystemDNS(t, probe, dns, "tunnel")
	verifyIdentity(t, started)
	refuseDuplicate(t, profile, runtimes, started)
	failPublic(t, probe, command, done, started, failure)
	finishPublic(t, probe, initial, done, failure, dns)
}

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
