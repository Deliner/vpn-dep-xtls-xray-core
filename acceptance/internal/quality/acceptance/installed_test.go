package acceptance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestInstalledCollectorBindsGenerationRuntimeAndPrivateContext(t *testing.T) {
	for _, damage := range []string{"", "runtime", "client", "profile", "binding", "identity", "unit-runtime", "override", "extra", "cleanup", "original-inputs", "missing-release"} {
		t.Run(damage, func(t *testing.T) {
			mode := installedEvidence(t)
			damageInstalledEvidence(t, mode, damage)
			data, err := installedCollector(mode, "tunnel").CombinedOutput()
			if (err == nil) != (damage == "") {
				t.Fatalf("installed evidence acceptance mismatch %s: %v %s", damage, err, data)
			}
		})
	}
}

func installedCollector(mode, policy string) *exec.Cmd {
	command := exec.Command("bash", "-c", `set -euo pipefail; source "$1"; source "$2"; if client_verify_installed "$3" "$4"; then exit 0; else exit 1; fi`, "fixture",
		"ops/testing/acceptance/client/verify.sh", "ops/testing/acceptance/client/verify-installed.sh", mode, policy)
	command.Dir = "../../.."
	return command
}

func installedEvidence(t *testing.T) string {
	t.Helper()
	mode := t.TempDir()
	hashes := map[string]string{}
	for _, name := range []string{"nhop-client", "xray", "sing-box", "libcronet.so", "profile.json"} {
		path := filepath.Join(mode, "bin", name)
		if name == "profile.json" {
			path = filepath.Join(mode, "installed-profile.json")
		}
		data := "synthetic installed input " + name
		write(t, path, data, 0600)
		hashes[name] = safeio.Hash([]byte(data))
	}
	identity := map[string]any{"schema_version": 1, "hashes": hashes, "network": map[string]any{
		"interface": "nhop0", "engine_uid": 0, "ipv6": "tunnel", "dns_ip": "198.18.0.2", "dns_name": "dns.example.invalid", "mtu": 1280}}
	id, err := safeio.Digest(identity)
	if err != nil {
		t.Fatal(err)
	}
	directory := "/var/lib/nhop-client/releases/" + id
	unit := installedUnitFixture(directory, id)
	write(t, filepath.Join(mode, "systemd-unit.txt"), unit, 0600)
	write(t, filepath.Join(mode, "systemd-unit.sha256"), safeio.Hash([]byte(unit))+"  /etc/systemd/system/nhop-client.service\n", 0600)
	fixtureJSON(t, filepath.Join(mode, "installed-release.json"), map[string]any{"identity": identity, "unit_sha256": safeio.Hash([]byte(unit))})
	fixtureJSON(t, filepath.Join(mode, "installed-owner.json"), map[string]any{"schema_version": 1, "token": strings.Repeat("a", 32)})
	installedInventoryFixture(t, mode, directory, hashes)
	write(t, filepath.Join(mode, "systemd-override.txt"), installedOverrideFixture, 0600)
	log := strings.ReplaceAll(serviceEvidence, "TestKernelSystemdManagedRecovery", "TestKernelInstalledServiceManagedRecovery")
	log = strings.Replace(log, "--- PASS:", "installed service: actual CLI, immutable runtime/profile snapshot, TCP/UDP/IPv6/DNS and retained guard recovery verified\ninstalled fixture removed: unit, generation and isolation drop-in absent\n--- PASS:", 1)
	write(t, filepath.Join(mode, "guest.log"), "original installation profile/inventory removed before service start\n"+log, 0600)
	return mode
}

func installedInventoryFixture(t *testing.T, mode, directory string, hashes map[string]string) {
	t.Helper()
	value := map[string]any{"schema_version": 1}
	for key, name := range map[string]string{"xray": "xray", "sing_box": "sing-box", "cronet": "libcronet.so"} {
		value[key] = map[string]string{"path": directory + "/" + name, "sha256": hashes[name]}
	}
	fixtureJSON(t, filepath.Join(mode, "installed-runtimes.json"), value)
	snapshot := map[string]string{}
	for name, digest := range hashes {
		snapshot[directory+"/"+name] = digest
	}
	for source, name := range map[string]string{"installed-owner.json": "/var/lib/nhop-client/owner.json", "installed-release.json": directory + "/release.json",
		"installed-runtimes.json": directory + "/runtimes.json", "systemd-unit.txt": directory + "/unit"} {
		digest, err := safeio.FileHash(filepath.Join(mode, source))
		if err != nil {
			t.Fatal(err)
		}
		snapshot[name] = digest
	}
	snapshot["/var/lib/nhop-client/install.lock"] = safeio.Hash(nil)
	fixtureJSON(t, filepath.Join(mode, "installed-snapshot.json"), snapshot)
}

func fixtureJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := safeio.JSON(path, value); err != nil {
		t.Fatal(err)
	}
}

func installedUnitFixture(directory, id string) string {
	return fmt.Sprintf("# nhop-client installation v1 %s %s\n[Service]\nExecStart=:\"%s/nhop-client\" \"service-run\" \"--profile\" \"%s/profile.json\" \"--runtimes\" \"%s/runtimes.json\" \"--ipv6\" \"tunnel\"\nExecStartPre=:\"%s/nhop-client\" \"service-prepare\"\nExecPaths=-/run/nhop-client\n", strings.Repeat("a", 32), id, directory, directory, directory, directory)
}

func damageInstalledEvidence(t *testing.T, mode, damage string) {
	t.Helper()
	path, old, replacement := "", "", ""
	switch damage {
	case "":
		return
	case "runtime", "client", "profile":
		path = map[string]string{"runtime": "bin/xray", "client": "bin/nhop-client", "profile": "installed-profile.json"}[damage]
		old, replacement = "synthetic", "changed"
	case "binding":
		path, old, replacement = "installed-runtimes.json", "/var/lib/nhop-client", "/tmp/foreign-client"
	case "identity":
		path, old, replacement = "installed-release.json", `"schema_version": 1`, `"schema_version": 2`
	case "unit-runtime":
		path, old, replacement = "systemd-unit.txt", `"service-run"`, `"foreign-command"`
	case "override":
		path, old, replacement = "systemd-override.txt", "/proc/45/ns/net", "/proc/1/ns/net"
	case "extra":
		path, old, replacement = "installed-snapshot.json", "{", `{ "extra": "unused",`
	case "cleanup", "original-inputs":
		path, old, replacement = "guest.log", map[string]string{"cleanup": "installed fixture removed:", "original-inputs": "original installation profile/inventory removed before service start"}[damage], "missing evidence"
	case "missing-release":
		if err := os.Remove(filepath.Join(mode, "installed-release.json")); err != nil {
			t.Fatal(err)
		}
		return
	}
	rewriteInstalledEvidence(t, mode, path, old, replacement)
}

func rewriteInstalledEvidence(t *testing.T, mode, path, old, replacement string) {
	t.Helper()
	file := filepath.Join(mode, path)
	data, err := os.ReadFile(file)
	if err != nil || !strings.Contains(string(data), old) {
		t.Fatal("invalid damaged evidence fixture", path, err)
	}
	changed := strings.ReplaceAll(string(data), old, replacement)
	write(t, file, changed, 0600)
	if path != "installed-runtimes.json" && path != "installed-release.json" {
		return
	}
	var snapshot map[string]string
	if err := safeio.LoadJSON(filepath.Join(mode, "installed-snapshot.json"), &snapshot); err != nil {
		t.Fatal(err)
	}
	for key := range snapshot {
		if filepath.Base(key) == strings.TrimPrefix(path, "installed-") {
			snapshot[key] = safeio.Hash([]byte(changed))
		}
	}
	fixtureJSON(t, filepath.Join(mode, "installed-snapshot.json"), snapshot)
}

func TestInstalledCollectorRecordedEvidence(t *testing.T) {
	mode := os.Getenv("NHOP_CLIENT_INSTALLED_EVIDENCE")
	if mode == "" {
		t.Skip("optional independent verification of recorded installed-service evidence")
	}
	if !filepath.IsAbs(mode) {
		t.Fatal("recorded fixture must be absolute")
	}
	for _, policy := range []string{"tunnel", "block"} {
		data, err := installedCollector(mode, policy).CombinedOutput()
		if err == nil {
			t.Logf("verified installed service policy %s", policy)
			return
		}
		t.Logf("policy %s not accepted: %s", policy, data)
	}
	t.Fatal("recorded installed service evidence was not accepted")
}

const installedOverrideFixture = `[Service]

# Test-only private namespace context; production settings above are unchanged.
NetworkNamespacePath=/proc/45/ns/net
BindPaths=/run/nhop-service-test-123/session/etc:/etc
BindPaths=/run/nhop-service-test-123/control:/run/nhop-client
BindPaths=/run/nhop-service-test-123/session:/run/nhop-session-test
BindPaths=/run/nhop-service-test-123/resolve:/run/systemd/resolve
BindPaths=/run/nhop-service-test-123/netif:/run/systemd/netif
Environment=DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/nhop-session-test/bus GORACE=atexit_sleep_ms=0
RuntimeMaxSec=90s
`
