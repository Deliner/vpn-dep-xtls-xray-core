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
	for _, damage := range []string{"", "missing-controller", "direct-leak", "counter-gap", "short-log", "empty-canary", "changed-unit", "cleanup-unit", "wrong-policy"} {
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
			if (err == nil) != (damage == "") {
				t.Fatalf("evidence acceptance mismatch: %s: %v %s", damage, err, data)
			}
		})
	}
}

func damageServiceEvidence(damage, log, unit string) (string, string) {
	switch damage {
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
system DNS tunnel: direct=2->2 HTTPS=2->4
systemd recovered native; MainPID=123
system DNS tunnel: direct=2->2 HTTPS=4->6
systemd recovered frontend; MainPID=234
system DNS tunnel: direct=2->2 HTTPS=6->8
systemd recovered controller; MainPID=345
system DNS tunnel: direct=2->2 HTTPS=8->10
systemd recovered restart; MainPID=456
system DNS tunnel: direct=2->2 HTTPS=10->12
system DNS blocked: direct=2->2 HTTPS=12->12
systemd direct UDP: attempts=9000 delivered=0; same firewall lease retained
system DNS direct: direct=2->3 HTTPS=12->12
--- PASS: TestKernelSystemdManagedRecovery (18.15s)
PASS
`
