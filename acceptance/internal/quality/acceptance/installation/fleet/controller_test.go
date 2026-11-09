package fleet

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type controllerEvidence struct {
	before, after map[string]any
	proof         map[string]any
	assignment    map[string][]byte
}

func TestFleetCollectorRejectsInvalidControllerProcessRecovery(t *testing.T) {
	for _, damage := range []string{"", "signal", "pid", "vpn", "agent", "authority", "credential", "desired", "revocation", "assignment", "denial", "heartbeat", "offline", "missing", "symlink"} {
		t.Run(damage, func(t *testing.T) {
			fixture := newControllerEvidence(t)
			fixture.damage(damage)
			directory := t.TempDir()
			fixture.write(t, directory)
			path := filepath.Join(directory, "controller-crash-exit.json")
			if damage == "missing" || damage == "symlink" {
				if err := os.Rename(path, path+".saved"); err != nil {
					t.Fatal(err)
				}
				if damage == "symlink" {
					if err := os.Symlink(path+".saved", path); err != nil {
						t.Fatal(err)
					}
				}
			}
			command := exec.Command("bash", "-c", `source "$1"; if fleet_verify_controller "$2"; then exit 0; else exit 1; fi`,
				"fixture", "../../../../../ops/testing/acceptance/fleet/controller.sh", directory)
			if output, err := command.CombinedOutput(); (err == nil) != (damage == "") {
				t.Fatalf("controller recovery evidence mismatch: %v %s", err, output)
			}
		})
	}
}

func newControllerEvidence(t *testing.T) controllerEvidence {
	t.Helper()
	generation := strings.Repeat("c", 64)
	desired := map[string]any{"revision": "5", "generation": generation, "payload_sha256": strings.Repeat("b", 64)}
	status := map[string]any{"phase": "ready", "attempt": "10", "accepted_revision": "5", "applied_revision": "5", "generation": generation}
	hashes := map[string]any{}
	for _, name := range []string{"admin", "agent_credentials", "authority", "executable", "tls_certificate", "tls_key"} {
		hashes[name] = strings.Repeat("a", 64)
	}
	before := map[string]any{"pid": 100, "agent_pid": "300", "vpn_pid": "400", "hashes": hashes,
		"active":                  map[string]any{"fleet_id": "acceptance", "node_id": "fleet-node", "enrolled": true, "revoked": false, "desired": desired, "status": status},
		"revoked":                 map[string]any{"fleet_id": "acceptance", "node_id": "revoked-control", "enrolled": true, "revoked": true},
		"revoked_assignment_http": 403, "revoked_report_http": 403, "revoked_enroll_http": 403}
	var after map[string]any
	if err := json.Unmarshal(crashJSON(t, before), &after); err != nil {
		t.Fatal(err)
	}
	after["pid"] = 200
	after["active"].(map[string]any)["status"].(map[string]any)["attempt"] = "11"
	return controllerEvidence{before: before, after: after,
		proof:      map[string]any{"pid": 100, "signal": "SIGKILL", "signal_number": 9, "signaled": true, "offline_verified": true, "recovered": true},
		assignment: map[string][]byte{"before": []byte(`{"fixture":"signed-envelope-bytes"}`), "after": []byte(`{"fixture":"signed-envelope-bytes"}`)}}
}

func (f controllerEvidence) damage(damage string) {
	switch damage {
	case "signal":
		f.proof["signal_number"] = 15
	case "pid":
		f.after["pid"] = 100
	case "vpn", "agent":
		f.after[damage+"_pid"] = "500"
	case "authority":
		f.after["hashes"].(map[string]any)["authority"] = strings.Repeat("d", 64)
	case "credential":
		f.after["hashes"].(map[string]any)["agent_credentials"] = strings.Repeat("d", 64)
	case "desired":
		f.after["active"].(map[string]any)["desired"].(map[string]any)["payload_sha256"] = strings.Repeat("d", 64)
	case "revocation":
		f.after["revoked"].(map[string]any)["revoked"] = false
	case "assignment":
		f.assignment["after"] = []byte(`{"fixture":"different-signed-envelope"}`)
	case "denial":
		f.after["revoked_report_http"] = 204
	case "heartbeat":
		f.after["active"].(map[string]any)["status"].(map[string]any)["attempt"] = "10"
	case "offline":
		f.proof["offline_verified"] = false
	}
}

func (f controllerEvidence) write(t *testing.T, directory string) {
	t.Helper()
	for name, value := range map[string]map[string]any{"before": f.before, "after": f.after} {
		digest := fmt.Sprintf("%x", sha256.Sum256(f.assignment[name]))
		value["assignment_sha256"] = digest
		value["active"].(map[string]any)["desired"].(map[string]any)["envelope_sha256"] = digest
		raw := crashJSON(t, value)
		f.proof[name+"_sha256"] = fmt.Sprintf("%x", sha256.Sum256(raw))
		for file, data := range map[string][]byte{"controller-" + name + ".json": raw, "controller-" + name + "-assignment.json": f.assignment[name]} {
			if err := os.WriteFile(filepath.Join(directory, file), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	report := reportFixture()
	report["agent_sha256"], report["controller_crash"] = strings.Repeat("a", 64), f.proof
	files := map[string][]byte{"fleet.json": crashJSON(t, report), "controller-crash-exit.json": crashJSON(t, f.proof),
		"controller-1.log": {}, "controller-2.log": {}, "controller-3.log": {}}
	for name, raw := range files {
		if err := os.WriteFile(filepath.Join(directory, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
