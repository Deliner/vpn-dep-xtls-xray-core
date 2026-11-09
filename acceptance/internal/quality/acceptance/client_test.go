package acceptance

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdCollectorRejectsMissingRecoveryLeaksAndChangedUnits(t *testing.T) {
	for _, damage := range []string{"", "released-gap", "missing-controller", "direct-leak", "counter-gap", "short-log", "empty-canary", "changed-unit", "cleanup-unit", "wrong-policy", "missing-disconnect", "early-disconnect", "disconnect-gap", "missing-remote", "duplicate-remote", "remote-counter", "remote-duration", "remote-attempts", "remote-failed"} {
		t.Run(damage, func(t *testing.T) {
			directory := t.TempDir()
			log := serviceEvidence
			unit := "[Service]\nExecStart=:/opt/nhop service-run \"--ipv6\" \"tunnel\"\nExecStartPre=:/opt/nhop service-prepare\nExecPaths=-/run/nhop-client\n"
			changedLog, changedUnit := damageServiceEvidence(damage, log, unit)
			if damage == "cleanup-unit" {
				unit = changedUnit
			}
			write(t, filepath.Join(directory, "guest.log"), changedLog, 0600)
			write(t, filepath.Join(directory, "systemd-unit.txt"), changedUnit, 0600)
			write(t, filepath.Join(directory, "systemd-unit.sha256"), fmt.Sprintf("%x  /run/systemd/system/nhop-client-acceptance-123.service\n", sha256.Sum256([]byte(unit))), 0600)
			script, err := filepath.Abs("../../../ops/testing/acceptance/client/verify.sh")
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command("bash", "-c", `set -euo pipefail; source "$1"; client_verify_systemd "$2" "$3"`, "fixture", script, directory, "tunnel")
			command.Env = os.Environ()
			data, err := command.CombinedOutput()
			if (err == nil) != (damage == "" || damage == "released-gap") {
				t.Fatalf("evidence acceptance mismatch: %s: %v %s", damage, err, data)
			}
		})
	}
}

func damageServiceEvidence(damage, log, unit string) (string, string) {
	switch damage {
	case "missing-remote":
		log = strings.ReplaceAll(log, "remote outage mode=drop", "missing outage mode=drop")
	case "duplicate-remote":
		log = strings.ReplaceAll(log, "mode=reject", "mode=drop")
	case "remote-counter":
		log = strings.ReplaceAll(log, "blocked_udp=13", "blocked_udp=0")
	case "remote-duration":
		log = strings.ReplaceAll(log, "unavailable_ms=8000", "unavailable_ms=50")
	case "remote-attempts":
		log = strings.ReplaceAll(log, "direct_attempts=500", "direct_attempts=0")
	case "remote-failed":
		log = strings.ReplaceAll(log, "--- PASS: TestKernelSystemdManagedRecovery/remote-drop", "--- FAIL: TestKernelSystemdManagedRecovery/remote-drop")
	case "released-gap":
		log = strings.ReplaceAll(log, "system DNS direct: direct=2->3", "system DNS direct: direct=3->4")
	case "missing-disconnect":
		log = strings.ReplaceAll(log, "system DNS disconnect requested:", "missing release:")
	case "early-disconnect":
		log = strings.ReplaceAll(log, "system DNS tunnel: direct=2->2 HTTPS=2->4", "system DNS disconnect requested: direct=2 HTTPS=2\nsystem DNS tunnel: direct=2->2 HTTPS=2->4")
	case "disconnect-gap":
		log = strings.ReplaceAll(log, "disconnect requested: direct=2", "disconnect requested: direct=3")
	case "missing-controller":
		log = strings.ReplaceAll(log, "systemd recovered controller;", "skipped controller;")
	case "direct-leak":
		log = strings.ReplaceAll(log, "direct=2->2 HTTPS=2->4", "direct=2->3 HTTPS=2->4")
	case "counter-gap":
		log = strings.ReplaceAll(log, "direct=2->2 HTTPS=2->4", "direct=3->3 HTTPS=2->4")
	case "short-log":
		log = strings.ReplaceAll(log, "system DNS blocked:", "missing DNS blocked:")
	case "empty-canary":
		log = strings.ReplaceAll(log, "attempts=9000", "attempts=0")
	case "changed-unit", "cleanup-unit":
		unit += "ExecStop=/bin/true\n"
	case "wrong-policy":
		log = strings.ReplaceAll(log, "IPv6=tunnel", "IPv6=block")
	}
	return log, unit
}

func TestClientEvidenceManifestMustCoverEveryRegularFile(t *testing.T) {
	for _, damage := range []string{"", "unhashed", "missing", "link", "escape"} {
		t.Run(damage, func(t *testing.T) {
			root := t.TempDir()
			directory := filepath.Join(root, "report")
			write(t, filepath.Join(directory, "record"), "evidence", 0600)
			manifest := fmt.Sprintf("%x  ./record\n", sha256.Sum256([]byte("evidence")))
			switch damage {
			case "unhashed":
				write(t, filepath.Join(directory, "unhashed"), "extra", 0600)
			case "missing":
				manifest += fmt.Sprintf("%x  ./missing\n", sha256.Sum256(nil))
			case "link":
				if err := os.Symlink("record", filepath.Join(directory, "link")); err != nil {
					t.Fatal(err)
				}
			case "escape":
				manifest = strings.ReplaceAll(manifest, "./record", "../record")
			}
			write(t, filepath.Join(directory, "evidence.sha256"), manifest, 0600)
			command := exec.Command("bash", "-c", `set -euo pipefail; source "$1"; client_verify_manifest "$2" "$3"`, "fixture",
				"../../../ops/testing/acceptance/client/verify-inputs.sh", directory, filepath.Join(root, "listing"))
			data, err := command.CombinedOutput()
			if (err == nil) != (damage == "") {
				t.Fatalf("manifest acceptance mismatch: %s: %v %s", damage, err, data)
			}
		})
	}
}

const serviceEvidence = `native client acceptance: linux/amd64; private mount and network namespaces
systemd policy IPv6=tunnel: systemd 255
system DNS direct: direct=0->2 HTTPS=0->0
system DNS tunnel: direct=2->2 HTTPS=0->2
system DNS blocked: direct=2->2 HTTPS=2->2
system DNS blocked: direct=2->2 HTTPS=2->2
system DNS tunnel: direct=2->2 HTTPS=2->4
remote outage mode=drop blocked_tcp=42 blocked_udp=13 unavailable_ms=8000 recovery_ms=10 direct_attempts=500; client PIDs, restart count, routes and guard retained
--- PASS: TestKernelSystemdManagedRecovery/remote-drop (8.10s)
system DNS blocked: direct=2->2 HTTPS=4->4
system DNS blocked: direct=2->2 HTTPS=4->4
system DNS tunnel: direct=2->2 HTTPS=4->6
remote outage mode=reject blocked_tcp=42 blocked_udp=13 unavailable_ms=8000 recovery_ms=10 direct_attempts=500; client PIDs, restart count, routes and guard retained
--- PASS: TestKernelSystemdManagedRecovery/remote-reject (8.10s)
system DNS tunnel: direct=2->2 HTTPS=6->8
systemd recovered native; MainPID=123
system DNS tunnel: direct=2->2 HTTPS=8->10
systemd recovered frontend; MainPID=234
system DNS tunnel: direct=2->2 HTTPS=10->12
systemd recovered controller; MainPID=345
system DNS tunnel: direct=2->2 HTTPS=12->14
systemd recovered restart; MainPID=456
system DNS tunnel: direct=2->2 HTTPS=14->16
system DNS blocked: direct=2->2 HTTPS=16->16
systemd direct UDP: attempts=9000 delivered=0; same firewall lease retained
system DNS disconnect requested: direct=2 HTTPS=16
system DNS direct: direct=2->3 HTTPS=16->16
--- PASS: TestKernelSystemdManagedRecovery (18.15s)
PASS
`

func TestBootCollectorRejectsMissingTransitionsLeaksAndUnprovenCounters(t *testing.T) {
	changes := map[string][2]string{
		"valid": {"unused", "unused"}, "missing-case": {"/before ", "/omitted "},
		"missing-observation": {"mode=after direct", "missing=after direct"},
		"duplicate-phase":     {"mode=after", "mode=before"}, "unknown-phase": {"mode=after", "mode=unknown"},
		"short-watch": {"attempts=200", "attempts=10"}, "leak": {"delivered=0", "delivered=1"},
		"missing-DNS":  {"system DNS blocked:", "missing DNS blocked:"},
		"wrong-policy": {"IPv6=tunnel", "IPv6=block"}, "failed-root": {"--- PASS: TestKernelBootProtectionHandoff (", "--- FAIL: TestKernelBootProtectionHandoff ("},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			log := strings.ReplaceAll(bootEvidence(), change[0], change[1])
			write(t, filepath.Join(directory, "guest.log"), log, 0600)
			command := exec.Command("bash", "-c", `set -euo pipefail; source "${1%/*}/verify.sh"; source "$1"; if client_verify_boot "$2" tunnel; then exit 0; else exit 1; fi`,
				"fixture", "../../../ops/testing/acceptance/client/verify-boot.sh", directory)
			output, err := command.CombinedOutput()
			if (err == nil) != (name == "valid") {
				t.Fatalf("bootstrap evidence mismatch %s: %v %s", name, err, output)
			}
		})
	}
}

func bootEvidence() string {
	var log strings.Builder
	log.WriteString("bootstrap policy: IPv6=tunnel\n")
	for _, mode := range []string{"stop", "handoff", "before", "after"} {
		fmt.Fprintf(&log, "--- PASS: TestKernelBootProtectionHandoff/%s (1s)\nbootstrap mode=%s direct UDP: attempts=200 delivered=0\n", mode, mode)
	}
	for i := range 8 {
		fmt.Fprintf(&log, "system DNS direct: direct=%d->%d HTTPS=0->0\n", i, i+1)
	}
	log.WriteString("system DNS tunnel: direct=8->8 HTTPS=0->1\n")
	for range 4 {
		log.WriteString("system DNS blocked: direct=8->8 HTTPS=1->1\n")
	}
	log.WriteString("--- PASS: TestKernelBootProtectionHandoff (8s)\nPASS\n")
	return log.String()
}

func TestRegistrationCollectorRequiresEachLifecycleAndFailureControl(t *testing.T) {
	for _, kind := range []string{"install", "boot-order"} {
		for name, change := range registrationDamage(kind) {
			t.Run(kind+"/"+name, func(t *testing.T) {
				directory := t.TempDir()
				log := strings.ReplaceAll(registrationEvidence(kind), change[0], change[1])
				write(t, filepath.Join(directory, "guest.log"), log, 0600)
				command := exec.Command("bash", "-c", `set -euo pipefail; source "$1"; if client_verify_registration "$2" "$3"; then exit 0; else exit 1; fi`,
					"fixture", "../../../ops/testing/acceptance/client/verify-registration.sh", kind, directory)
				output, err := command.CombinedOutput()
				if (err == nil) != (name == "valid") {
					t.Fatalf("registration evidence mismatch: %v %s", err, output)
				}
			})
		}
	}
}

func registrationDamage(kind string) map[string][2]string {
	changes := map[string][2]string{
		"valid": {"unused", "unused"}, "failed-root": {"--- PASS:", "--- FAIL:"},
		"missing-completion": {"\nPASS\n", "\n"}, "duplicate-completion": {"\nPASS\n", "\nPASS\nPASS\n"},
		"race": {"\nPASS\n", "\nWARNING: DATA RACE\nPASS\n"},
	}
	if kind == "install" {
		changes["missing-disable"] = [2]string{"service-disable success=true", "missing-disable success=true"}
		changes["missing-refusal"] = [2]string{"service-remove success=false", "service-remove success=true"}
		changes["same-generation"] = [2]string{strings.Repeat("b", 64), strings.Repeat("a", 64)}
		changes["short-generation"] = [2]string{strings.Repeat("a", 64), "a"}
		changes["missing-registration"] = [2]string{"boot registration verified:", "boot registration omitted:"}
	} else {
		changes["missing-weak-control"] = [2]string{"control: Wants+After", "omitted: Wants+After"}
		changes["missing-failure"] = [2]string{"required protection:", "omitted protection:"}
		changes["duplicate-failure"] = [2]string{"required protection:", "required protection: failure blocks dependent activation\nrequired protection:"}
		changes["missing-retained-firewall"] = [2]string{"unit stop retains firewall", "unit stop clears firewall"}
		changes["missing-explicit-stop"] = [2]string{"explicit client stop restores connectivity", "omitted explicit stop"}
	}
	return changes
}

func registrationEvidence(kind string) string {
	if kind == "boot-order" {
		return `control: Wants+After permits dependent activation after protection failure
required protection: failure blocks dependent activation
native systemd verified: successful protection precedes dependent; unit stop retains firewall; explicit client stop restores connectivity
--- PASS: TestKernelBootOrderingFailsClosed (2s)
PASS
`
	}
	return fmt.Sprintf(`actual CLI service-enable success=false: supported network managers:
actual CLI service-enable success=true: {}
actual CLI service-enable success=true: {}
actual CLI service-remove success=false: client boot registration is retained;
actual CLI service-disable success=true: {}
actual CLI service-disable success=true: {}
boot registration verified: disabled by default, enable/idempotent/dependencies/unregister refusal/disable; no service activation
installation verified: initial/idempotent/update/rollback/interrupted reload/remove; generations %s %s retained
--- PASS: TestKernelClientServiceInstallation (4s)
PASS
`, strings.Repeat("a", 64), strings.Repeat("b", 64))
}
