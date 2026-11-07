package network

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKernelTUNTrafficDNSAndEngineFailure(t *testing.T) {
	if os.Getenv("NHOP_CLIENT_KERNEL_TEST") != "private-netns" || os.Getenv("NHOP_CLIENT_RUNTIME") == "" {
		t.Skip("requires isolated Linux guest and verified candidate engines")
	}
	hostname, err := os.Hostname()
	self, selfErr := os.Readlink("/proc/self/ns/net")
	initial, initialErr := os.Readlink("/proc/1/ns/net")
	if err != nil || selfErr != nil || initialErr != nil || hostname != "lima-payload" || self == initial || os.Geteuid() != 0 {
		t.Fatal("live TUN test refused outside isolated owned Linux guest")
	}
	directory := liveDirectory(t)
	liveConfigs(t, directory, startTunneledDNS(t))
	kernelTCP(t, "198.18.0.2:18080")
	kernelUDP(t, "198.18.0.2:18081")
	kernelTCP(t, "[2001:db8:feed::2]:18080")
	liveProcess(t, directory, "exit", filepath.Join(directory, "xray"), "run", "-config", filepath.Join(directory, "server.json"))
	applyKernelPolicy(t)
	native := liveProcess(t, directory, "native", "ip", "netns", "exec", "nhop-client-inner", "setpriv", "--reuid=20000", "--regid=20000", "--clear-groups",
		filepath.Join(directory, "xray"), "run", "-config", filepath.Join(directory, "native.json"))
	waitLiveCommand(t, "12080", "ss", "-H", "-lnt", "sport", "=", ":12080")
	frontend := liveProcess(t, directory, "tun", "ip", "netns", "exec", "nhop-client-inner", filepath.Join(directory, "sing-box"), "run", "-c", filepath.Join(directory, "tun.json"))
	waitLiveCommand(t, "nhop0", "ip", "route", "get", "198.18.0.2")
	probe := startKernelProbe(t, 20001)
	checkTUNTraffic(t, probe, true)
	native()
	checkTUNTraffic(t, probe, false)
	frontend()
	checkTUNTraffic(t, probe, false)
}

func checkTUNTraffic(t *testing.T, probe *kernelProbe, allowed bool) {
	t.Helper()
	probe.check(t, kernelRequest{"tcp", "198.18.0.2:18080", ""}, allowed)
	probe.check(t, kernelRequest{"udp", "198.18.0.2:18081", ""}, allowed)
	probe.check(t, kernelRequest{"tcp", "[2001:db8:feed::2]:18080", ""}, allowed)
	probe.check(t, kernelRequest{"udp", DNS + ":53", "dns"}, allowed)
}
