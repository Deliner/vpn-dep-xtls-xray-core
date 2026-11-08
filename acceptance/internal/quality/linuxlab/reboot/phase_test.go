package reboot

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func TestRebootAcceptance(t *testing.T) {
	f := hostFixture(t)
	f.identify(t)
	f.materials(t)
	f.stage(t, "boot-exit")
	f.stage(t, "boot-client")
	f.phase(t, "baseline")
	f.guest(t, "boot-client", "TestRebootBaseline")
	f.guest(t, "boot-client", "TestRebootRecovery")
	for _, name := range []string{"healthy", "guard-failure"} {
		if !t.Run(name, func(t *testing.T) { f.reboot(t, name) }) {
			return
		}
	}
}

func (f *fixture) reboot(t *testing.T, name string) {
	t.Helper()
	f.phase(t, name)
	before := f.metrics(t, name+"-before")
	f.guest(t, "boot-client", "TestRebootArm")
	f.units(t, name)
	raw, err := f.attempt(15*time.Second, "shell", "--workdir", "/home/nhop", "boot-client", "sudo", "--", "systemctl", "reboot")
	require(t, safeio.Atomic(filepath.Join(f.root, name+"-reboot-command.log"), raw, 0600))
	t.Logf("reboot request result=%v; waiting for a changed kernel boot ID and local recovery receipt", err)
	result := f.await(t, name)
	f.journal(t, "boot-client", name)
	canary := decode[contract.Canary](t, f.save(t, "boot-client", contract.Root+"/canary-"+name+".json", "canary-"+name+".json"))
	after := f.metrics(t, name+"-after")
	f.verify(t, name, before, after, result, canary)
	f.guest(t, "boot-client", "TestRebootBaseline")
	t.Logf("real reboot %s: boot %s -> %s, NIC attempts=%d, receiver deliveries=%d", name, result.BeforeBootID, result.BootID, canary.Attempts, after.Canary[name]-before.Canary[name])
}

func (f *fixture) await(t *testing.T, name string) contract.Recovery {
	t.Helper()
	deadline := time.Now().Add(5 * time.Minute)
	var latest []byte
	for time.Now().Before(deadline) {
		raw, err := f.attempt(8*time.Second, "shell", "--workdir", "/home/nhop", "boot-client", "sudo", "--", "cat", contract.Root+"/recovery-"+name+".json")
		latest = raw
		var result contract.Recovery
		if err == nil && json.Unmarshal(raw, &result) == nil && result.Phase == name && result.BeforeBootID == f.boots["boot-client"] && result.BootID != result.BeforeBootID {
			require(t, safeio.Atomic(filepath.Join(f.root, "boot-client-recovery-"+name+".json"), raw, 0600))
			return result
		}
		time.Sleep(2 * time.Second)
	}
	require(t, safeio.Atomic(filepath.Join(f.root, name+"-recovery-timeout.log"), latest, 0600))
	t.Fatal("local recovery receipt unavailable after five minutes; retained VM for inspection")
	return contract.Recovery{}
}

func (f *fixture) verify(t *testing.T, name string, before, after contract.PeerMetrics, result contract.Recovery, canary contract.Canary) {
	t.Helper()
	if result.Status != "passed" || result.Error != "" || result.CleanupError != "" {
		t.Fatalf("boot acceptance/recovery failed: %+v", result)
	}
	if canary.Phase != name || canary.BootID != result.BootID || canary.Attempts < 20 || canary.Finished == "" {
		t.Fatal("incomplete early NIC canary evidence")
	}
	start, err := time.Parse(time.RFC3339Nano, canary.Started)
	require(t, err)
	end, err := time.Parse(time.RFC3339Nano, canary.Finished)
	require(t, err)
	if end.Sub(start) < 30*time.Second || before.Canary["baseline"] < 40 || after.Canary[name] != before.Canary[name] {
		t.Fatal("direct delivery or insufficient positively controlled observation window")
	}
	if name == "healthy" && (after.TunnelDNS <= before.TunnelDNS || after.DirectDNS != before.DirectDNS) {
		t.Fatal("system DNS did not exclusively reach the tunneled resolver")
	}
	if name == "healthy" {
		query, answer, ok := strings.Cut(result.Observations["system_dns"], "=")
		if !ok || answer != f.config["boot-exit"].PeerIP || after.DNSQueries[query] <= before.DNSQueries[query] {
			t.Fatal("system DNS probe has no matching peer-side query evidence")
		}
	}
	verifyOrder(t, result)
}
