package reboot

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func verifyOrder(t *testing.T, manager string, result contract.Recovery) {
	t.Helper()
	values := result.Observations
	canary := monotonic(t, values[contract.CanaryUnit+".properties"], "ActiveEnterTimestampMonotonic")
	guardStart := monotonic(t, values["nhop-client-boot.service.properties"], "ExecMainStartTimestampMonotonic")
	guardDone := monotonic(t, values["nhop-client-boot.service.properties"], "ExecMainExitTimestampMonotonic")
	network := monotonic(t, values[manager+".properties"], "ExecMainStartTimestampMonotonic")
	client := monotonic(t, values["nhop-client.service.properties"], "ExecMainStartTimestampMonotonic")
	if canary == 0 || guardStart < canary || guardDone < guardStart {
		t.Fatal("early probe/guard ordering was not enforced")
	}
	if result.Phase == "healthy" && (network < guardDone || client < guardDone) {
		t.Fatal("network/client started before protection completed")
	}
	if result.Phase == "guard-failure" && (network != 0 || client != 0) {
		t.Fatal("failed guard allowed a dependent process to execute")
	}
}

func monotonic(t *testing.T, properties, name string) uint64 {
	t.Helper()
	for _, line := range strings.Split(properties, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok && key == name {
			parsed, err := strconv.ParseUint(value, 10, 64)
			require(t, err)
			return parsed
		}
	}
	t.Fatalf("missing systemd monotonic property: %s", name)
	return 0
}

func (f *fixture) metrics(t *testing.T, name string) contract.PeerMetrics {
	t.Helper()
	address := "http://" + f.config["boot-exit"].PeerIP + ":18080/metrics?token=" + f.config["boot-exit"].Token
	raw := f.remote(t, "boot-exit", "curl", "--silent", "--show-error", "--fail", "--max-time", "5", address)
	require(t, safeio.Atomic(filepath.Join(f.root, "peer-"+name+".json"), raw, 0600))
	return decode[contract.PeerMetrics](t, raw)
}

func (f *fixture) units(t *testing.T, phase string) {
	t.Helper()
	names := []string{"nhop-client.service", "nhop-client-boot.service", f.config["boot-client"].NetworkManager, contract.CanaryUnit, contract.RecoveryUnit, contract.RecoveryTimer}
	raw := f.remote(t, "boot-client", append([]string{"systemctl", "cat"}, names...)...)
	require(t, safeio.Atomic(filepath.Join(f.root, phase+"-units.txt"), raw, 0600))
	args := []string{"systemd-analyze", "verify"}
	for _, name := range names {
		args = append(args, name)
	}
	raw = f.remote(t, "boot-client", args...)
	require(t, safeio.Atomic(filepath.Join(f.root, phase+"-unit-verification.log"), raw, 0600))
}

func (f *fixture) journal(t *testing.T, role, phase string) {
	t.Helper()
	args := []string{"journalctl", "--no-pager", "-b", "-o", "short-monotonic"}
	for _, name := range []string{"nhop-client.service", "nhop-client-boot.service", f.config[role].NetworkManager, contract.CanaryUnit, contract.RecoveryUnit, contract.RecoveryTimer, contract.PeerUnit} {
		args = append(args, "-u", name)
	}
	raw := f.remote(t, role, args...)
	require(t, safeio.Atomic(filepath.Join(f.root, role+"-"+phase+"-journal.log"), raw, 0600))
}

func (f *fixture) clean(t *testing.T, role string) {
	t.Helper()
	boot := strings.TrimSpace(string(f.save(t, role, "/proc/sys/kernel/random/boot_id", "final-boot-id.txt")))
	if role == "boot-exit" && boot != f.boots[role] {
		t.Error("exit VM rebooted during client acceptance")
	}
	f.journal(t, role, "final")
	f.guest(t, role, "TestRebootClean")
	raw := f.remote(t, role, "tar", "-czf", "-", "--exclude=bin", "--exclude=reboot.test", "-C", contract.Root, ".")
	path := filepath.Join(f.root, role+"-guest-evidence.tar.gz")
	require(t, safeio.Create(path, raw, 0600))
	if t.Failed() {
		return
	}
	f.remote(t, role, "rm", "-rf", "--", contract.Root)
	f.command(t, "shell", "--workdir", "/home/nhop", role, "rmdir", "--", "/home/nhop/runs/reboot-"+filepath.Base(f.root))
}
