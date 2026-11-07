package network

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestKernelKillSwitchDeniesDirectAndEstablishedEgress(t *testing.T) {
	if os.Getenv("NHOP_CLIENT_KERNEL_TEST") != "private-netns" {
		t.Skip("requires an explicitly isolated owned Linux VM namespace")
	}
	hostname, err := os.Hostname()
	if err != nil || hostname != "lima-payload" || os.Geteuid() != 0 {
		t.Fatal("kernel policy test refused outside owned root Linux lab")
	}
	self, err := os.Readlink("/proc/self/ns/net")
	initial, initialErr := os.Readlink("/proc/1/ns/net")
	if err != nil || initialErr != nil || self == initial {
		t.Fatal("kernel policy test requires a private network namespace")
	}
	origins := kernelOrigins(t)
	ordinary, engine := startKernelProbe(t, 20001), startKernelProbe(t, 20000)
	for _, request := range kernelRequests() {
		ordinary.check(t, request, true)
	}
	ordinary.check(t, kernelRequest{"tcp", "198.51.100.7:24443", "hold"}, true)
	applyKernelPolicy(t, Defaults(20000))
	counts := make([]int64, len(origins))
	for i, counter := range origins {
		counts[i] = counter.Load()
	}
	for _, request := range kernelRequests() {
		ordinary.check(t, request, false)
	}
	ordinary.check(t, kernelRequest{Action: "reuse"}, false)
	for i, counter := range origins {
		if counter.Load() != counts[i] {
			t.Fatalf("direct payload reached canary %d", i)
		}
	}
	verifyEngineKernelPolicy(t, engine)
}

func kernelOrigins(t *testing.T) []*atomic.Int64 {
	return []*atomic.Int64{kernelTCP(t, "198.51.100.7:24443"), kernelTCP(t, "198.51.100.7:24444"),
		kernelUDP(t, "198.51.100.7:24443"), kernelUDP(t, "198.51.100.7:53"),
		kernelTCP(t, "[2001:db8:ffff::1]:24444"), kernelUDP(t, "[2001:db8:ffff::1]:53")}
}

func kernelRequests() []kernelRequest {
	return []kernelRequest{{"tcp", "198.51.100.7:24443", ""}, {"tcp", "198.51.100.7:24444", ""},
		{"udp", "198.51.100.7:24443", ""}, {"udp", "198.51.100.7:53", ""},
		{"tcp", "[2001:db8:ffff::1]:24444", ""}, {"udp", "[2001:db8:ffff::1]:53", ""}}
}

func applyKernelPolicy(t *testing.T, options Options) {
	t.Helper()
	policy, err := Firewall(fixture(t), options)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "guard.nft")
	if err := os.WriteFile(path, []byte(policy), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("ip", "netns", "exec", "nhop-client-inner", "nft", "--file", path)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("kernel rejected generated nft policy: %v %s", err, output)
	}
}

func verifyEngineKernelPolicy(t *testing.T, engine *kernelProbe) {
	t.Helper()
	engine.check(t, kernelRequest{"tcp", "198.51.100.7:24443", ""}, true)
	engine.check(t, kernelRequest{"udp", "198.51.100.7:24443", ""}, true)
	engine.check(t, kernelRequest{"tcp", "198.51.100.7:24444", ""}, false)
	engine.check(t, kernelRequest{"udp", "198.51.100.7:53", ""}, false)
}
