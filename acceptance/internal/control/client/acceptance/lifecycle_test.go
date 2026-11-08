package acceptance

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"

	"vpn-script/internal/core/dag"
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

func TestKernelAllProtocolTUN(t *testing.T) {
	if os.Getenv("NHOP_CLIENT_PROTOCOL_TEST") != "private-netns" || os.Getenv("NHOP_CLIENT_BINARY") == "" {
		t.Skip("requires isolated Linux namespaces and the real public CLI binary")
	}
	checkIsolation(t)
	t.Logf("TUN matrix policy: IPv6=%s; real CLI import/start/reconnect/stop", os.Getenv("NHOP_CLIENT_IPV6"))
	origin(t, "198.18.0.2:18080")
	origin(t, "[2001:db8:feed::2]:18080")
	udpOrigin(t)
	dns := startDNSOrigins(t)
	for _, hops := range []int{1, 3} {
		t.Run(fmt.Sprintf("%d-hop", hops), func(t *testing.T) {
			fixture := protocolLab(t, hops)
			for _, name := range dag.Keys(fixture.topology.Routes) {
				t.Run(name, func(t *testing.T) { protocolLifecycle(t, fixture, name, dns) })
			}
		})
	}
}

func protocolLifecycle(t *testing.T, fixture *protocolFixture, name string, dns *dnsOrigins) {
	t.Helper()
	request := fixture.request(t, name)
	probe, initial := newController(t, request)
	checkSystemDNS(t, probe, dns, "direct")
	profile, runtimes := cliInputs(t, request)
	_, done, started := cliStart(t, profile, runtimes)
	if started.Error != "" || started.Status.Phase != "running" {
		t.Fatalf("protocol start: %+v", started)
	}
	activeTraffic(t, probe)
	checkUDP6(t, probe, os.Getenv("NHOP_CLIENT_IPV6") == "tunnel")
	checkSystemDNS(t, probe, dns, "tunnel")
	canary, watch := directCanary(t), startLeakWatch(t)
	if started.Status.NativePID <= 1 || syscall.Kill(started.Status.NativePID, syscall.SIGKILL) != nil {
		t.Fatal("cannot stop native protocol engine")
	}
	waitFailure(t)
	checkTraffic(t, probe, false)
	checkUDP6(t, probe, false)
	checkSystemDNS(t, probe, dns, "blocked")
	reconnectAndVerify(t, probe, started, dns)
	checkUDP6(t, probe, os.Getenv("NHOP_CLIENT_IPV6") == "tunnel")
	watch.stop(t)
	if watch.attempts < 20 || canary.Load() != 0 {
		t.Fatalf("protocol direct leak control: attempts=%d delivered=%d", watch.attempts, canary.Load())
	}
	t.Logf("TUN protocol=%s hops=%d IPv6=%s direct UDP: attempts=%d delivered=0; same firewall lease retained",
		name, request.Profile.Metadata.HopCount, os.Getenv("NHOP_CLIENT_IPV6"), watch.attempts)
	finishPublic(t, probe, initial, done, "graceful", dns)
	if result := probe.call(t, "probe-direct"); result.Error != "" || canary.Load() == 0 {
		t.Fatalf("protocol direct canary not reachable after explicit stop: %+v", result)
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
	for _, failure := range []string{"graceful", "native", "controller-term", "controller-kill", "reconnect-native", "reconnect-frontend", "resume-term", "resume-kill", "intent-before-resume", "intent-after-resume", "intent-before-stop", "intent-after-stop"} {
		t.Run(failure, func(t *testing.T) { publicLifecycle(t, failure, dns) })
	}
}

func publicLifecycle(t *testing.T, failure string, dns *dnsOrigins) {
	t.Helper()
	if strings.HasPrefix(failure, "intent-") {
		intentLifecycle(t, strings.TrimPrefix(failure, "intent-"), dns)
		return
	}
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
	if strings.HasPrefix(failure, "resume-") {
		for _, path := range []string{profile, runtimes} {
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		resumePublic(t, probe, initial, command, done, started, strings.TrimPrefix(failure, "resume-"), dns)
		return
	}
	failPublic(t, probe, command, done, started, failure)
	finishPublic(t, probe, initial, done, failure, dns)
}

func intentLifecycle(t *testing.T, failure string, dns *dnsOrigins) {
	t.Helper()
	request := fixture(t)
	probe, initial := newController(t, request)
	checkSystemDNS(t, probe, dns, "direct")
	profile, runtimes := cliInputs(t, request)
	stage, action, _ := strings.Cut(failure, "-")
	pending := crashGuard(t, profile, runtimes, stage)
	installed := verifyGuardIntent(t, pending, probe, stage)
	phase := "direct"
	if stage == "after" {
		phase = "blocked"
	}
	checkSystemDNS(t, probe, dns, phase)
	if action == "resume" {
		resumeGuardIntent(t, pending, probe, stage, installed, dns)
	} else if result := cliCall(t, "stop"); result.Error != "" || result.Status.Phase != "stopped" {
		t.Fatal("planned guard explicit stop failed", result)
	}
	checkTraffic(t, probe, true)
	checkSystemDNS(t, probe, dns, "direct")
	if probe.call(t, "dns").DNS != initial.DNS || probe.call(t, "routes").Routes != initial.Routes {
		t.Fatal("planned guard recovery did not restore baseline")
	}
	t.Logf("planned guard recovery verified: stage=%s action=%s", stage, action)
}
