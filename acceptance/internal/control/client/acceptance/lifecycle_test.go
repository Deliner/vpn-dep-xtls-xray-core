package acceptance

import (
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestKernelSessionStartStopAndFailures(t *testing.T) {
	if os.Getenv("NHOP_CLIENT_KERNEL_TEST") != "private-netns" || os.Getenv("NHOP_CLIENT_RUNTIME") == "" {
		t.Skip("requires isolated owned Linux VM and verified engines")
	}
	checkIsolation(t)
	origin(t, "198.18.0.2:18080")
	origin(t, "[2001:db8:feed::2]:18080")
	udpOrigin(t)
	for _, failure := range []string{"graceful", "native", "frontend"} {
		t.Run(failure, func(t *testing.T) { lifecycle(t, failure) })
	}
}

func lifecycle(t *testing.T, failure string) {
	t.Helper()
	c, initial := newController(t, fixture(t))
	started := c.call(t, "start")
	if started.Error != "" || started.Status.Phase != "running" {
		t.Fatalf("start: %+v", started)
	}
	checkTraffic(t, c, true)
	if state := c.call(t, "dns").DNS; !strings.Contains(state, "172.30.255.2") || !strings.Contains(state, "~.") {
		t.Fatalf("session did not configure its DNS link: %s", state)
	}
	if failure != "graceful" {
		failEngine(t, c, failure, started)
	}
	if result := c.call(t, "stop"); result.Error != "" || result.Status.Phase != "stopped" {
		t.Fatalf("explicit stop: %+v", result)
	}
	checkTraffic(t, c, true)
	if c.call(t, "dns").DNS != initial.DNS {
		t.Fatal("explicit stop did not restore original DNS state")
	}
	if after := c.call(t, "routes").Routes; after != initial.Routes {
		t.Fatalf("routing rules were not restored: before=%s after=%s", initial.Routes, after)
	}
}

func failEngine(t *testing.T, c *controller, failure string, started response) {
	t.Helper()
	pid := started.Status.NativePID
	if failure == "frontend" {
		pid = started.Status.FrontendPID
	}
	if pid <= 1 || syscall.Kill(pid, syscall.SIGKILL) != nil {
		t.Fatal("could not stop owned engine")
	}
	failed := c.call(t, "wait")
	t.Logf("firewall after engine failure: %s", c.call(t, "firewall").DNS)
	if failed.Error == "" || failed.Status.Phase != "failed-closed" {
		t.Fatalf("engine failure was not reported: %+v", failed)
	}
	checkTraffic(t, c, false)
}

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
	for _, failure := range []string{"graceful", "native", "controller-term", "controller-kill", "reconnect-native", "reconnect-frontend"} {
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
	if strings.HasPrefix(failure, "reconnect-") {
		reconnectPublic(t, probe, initial, done, started, strings.TrimPrefix(failure, "reconnect-"), dns)
		return
	}
	failPublic(t, probe, command, done, started, failure)
	finishPublic(t, probe, initial, done, failure, dns)
}
