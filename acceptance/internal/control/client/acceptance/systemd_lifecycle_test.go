package acceptance

import (
	"encoding/json"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	clientcontrol "vpn-script/internal/control/client/controller"
	"vpn-script/internal/platform/safeio"
)

func TestKernelSystemdManagedRecovery(t *testing.T) {
	if os.Getenv("NHOP_CLIENT_SYSTEMD_TEST") != "private-netns" {
		t.Skip("requires real owned Linux systemd and private networking, NSS and DNS")
	}
	request, probe, initial, dns := managedContext(t)
	service := newManagedService(t, probe, request)
	exerciseManaged(t, service, probe, initial, dns)
}

func exerciseManaged(t *testing.T, service managedService, probe *controller, initial response, dns *dnsOrigins) {
	t.Helper()
	started := waitManaged(t, clientcontrol.Diagnostic{})
	protectedDNS := dns.direct.Load()
	verifyIdentity(t, started)
	verifyManagedNamespace(t, probe, started.Status)
	activeTraffic(t, probe)
	checkSystemDNS(t, probe, dns, "tunnel")
	canary := directCanary(t)
	watch := startLeakWatch(t)
	for _, failure := range []string{"native", "frontend", "controller", "restart"} {
		started = recoverManaged(t, service, probe, started, failure, dns)
	}
	refuseManagedDrift(t, service, probe, started)
	serviceCommand(t, "start", service.name)
	started = waitManaged(t, started)
	checkSystemDNS(t, probe, dns, "tunnel")
	serviceCommand(t, "stop", service.name)
	checkTraffic(t, probe, false)
	checkSystemDNS(t, probe, dns, "blocked")
	watch.stop(t)
	if watch.attempts < 20 || canary.Load() != 0 {
		t.Fatalf("managed direct leak control: attempts=%d delivered=%d", watch.attempts, canary.Load())
	}
	t.Logf("systemd direct UDP: attempts=%d delivered=0; same firewall lease retained", watch.attempts)
	if dns.direct.Load() != protectedDNS {
		t.Fatal("direct DNS escaped during the protected service lifecycle")
	}
	finishManaged(t, service, probe, initial, dns)
}

func waitManaged(t *testing.T, previous clientcontrol.Diagnostic) clientcontrol.Diagnostic {
	t.Helper()
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		data, err := cliCommand(t, "diagnostics").Output()
		var current clientcontrol.Diagnostic
		if err == nil && json.Unmarshal(data, &current) == nil && current.Controller == "active" && current.Error == "" && current.Status.Phase == "running" && current.Status.NativePID != previous.Status.NativePID {
			if previous.Directory != "" && (current.Directory != previous.Directory || current.Status.Guard != previous.Status.Guard) {
				t.Fatal("systemd restart changed firewall ownership or session", current)
			}
			return current
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("managed controller did not become ready after restart")
	return clientcontrol.Diagnostic{}
}

func recoverManaged(t *testing.T, service managedService, probe *controller, previous clientcontrol.Diagnostic, failure string, dns *dnsOrigins) clientcontrol.Diagnostic {
	t.Helper()
	switch failure {
	case "native", "frontend":
		pid := previous.Status.NativePID
		if failure == "frontend" {
			pid = previous.Status.FrontendPID
		}
		if pid <= 1 || syscall.Kill(pid, syscall.SIGKILL) != nil {
			t.Fatal("cannot stop owned managed engine")
		}
	case "controller":
		serviceCommand(t, "kill", "--kill-whom=main", "--signal=KILL", service.name)
	case "restart":
		serviceCommand(t, "restart", service.name)
	}
	current := waitManaged(t, previous)
	verifyManagedNamespace(t, probe, current.Status)
	activeTraffic(t, probe)
	checkSystemDNS(t, probe, dns, "tunnel")
	t.Logf("systemd recovered %s; %s", failure, serviceCommand(t, "show", service.name, "-p", "MainPID", "-p", "NRestarts", "-p", "SubState"))
	return current
}

func refuseManagedDrift(t *testing.T, service managedService, probe *controller, started clientcontrol.Diagnostic) {
	t.Helper()
	serviceCommand(t, "stop", service.name)
	checkTraffic(t, probe, false)
	before := probe.call(t, "routes").Routes
	args := []string{"service-run", "--profile", service.profile, "--runtimes", service.runtimes, "--ipv6", os.Getenv("NHOP_CLIENT_IPV6"), "--dns-ip", "198.18.0.2", "--dns-name", "dns.example.invalid", "--mtu", "1400"}
	cliRejects(t, args...)
	if err := os.Chmod(service.profile, 0644); err != nil {
		t.Fatal(err)
	}
	cliRejects(t, args[:len(args)-2]...)
	info, err := os.Stat(service.profile)
	if err != nil || info.Mode().Perm() != 0644 {
		t.Fatal("service input was silently repaired", err)
	}
	if err := os.Chmod(service.profile, 0600); err != nil {
		t.Fatal(err)
	}
	current := cliCall(t, "diagnostics")
	if current.Status.Guard != started.Status.Guard || probe.call(t, "routes").Routes != before {
		t.Fatal("refused managed intent changed protection")
	}
	checkTraffic(t, probe, false)
}

func finishManaged(t *testing.T, service managedService, probe *controller, initial response, dns *dnsOrigins) {
	t.Helper()
	t.Logf("system DNS disconnect requested: direct=%d HTTPS=%d", dns.direct.Load(), dns.tunneled.Load())
	if result := cliCall(t, "stop"); result.Error != "" || result.Status.Phase != "stopped" {
		t.Fatal("explicit stop failed", result)
	}
	checkTraffic(t, probe, true)
	checkSystemDNS(t, probe, dns, "direct")
	if result := probe.call(t, "probe-direct"); result.Error != "" {
		t.Fatal("positive control: direct canary unreachable after stop", result)
	}
	if probe.call(t, "dns").DNS != initial.DNS || probe.call(t, "routes").Routes != initial.Routes {
		t.Fatal("managed stop failed to restore network baseline")
	}
	sessionLogs(t, cliCall(t, "diagnostics").Directory)
	// A fresh service can start after explicit disconnect; a control-socket stop
	// while managed must exit successfully and must not trigger Restart=on-failure.
	serviceCommand(t, "start", service.name)
	current := waitManaged(t, clientcontrol.Diagnostic{})
	if result := cliCall(t, "stop"); result.Error != "" {
		t.Fatal("active managed stop failed", result)
	}
	time.Sleep(2300 * time.Millisecond)
	if state := strings.TrimSpace(serviceCommand(t, "show", service.name, "--value", "-p", "ActiveState")); state != "inactive" {
		t.Fatal("explicit disconnect caused a service restart", state)
	}
	sessionLogs(t, current.Directory)
	if err := safeio.CheckAccess(service.profile, 0600, 0, 0); err != nil {
		t.Fatal(err)
	}
}
