package fleet

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type crashEvidence struct {
	files map[string]any
	proof map[string]any
}

func TestFleetCollectorRejectsSelfConsistentButInvalidCrashEvidence(t *testing.T) {
	for _, damage := range []string{"", "intent", "credentials", "signal", "phase", "revision", "attempt", "final-state", "killed-intent", "missing"} {
		t.Run(damage, func(t *testing.T) {
			fixture := newCrashEvidence(t)
			fixture.damage(damage)
			directory := t.TempDir()
			fixture.write(t, directory)
			if damage == "missing" {
				if err := os.Remove(filepath.Join(directory, "interrupted-final-state.json")); err != nil {
					t.Fatal(err)
				}
			}
			command := exec.Command("bash", "-c", `source "$1"; if fleet_verify_interruption "$2"; then exit 0; else exit 1; fi`,
				"fixture", "../../../../../ops/testing/acceptance/fleet/interruption.sh", directory)
			if output, err := command.CombinedOutput(); (err == nil) != (damage == "") {
				t.Fatalf("crash evidence mismatch: %v %s", err, output)
			}
		})
	}
}

func newCrashEvidence(t *testing.T) crashEvidence {
	t.Helper()
	receipt := map[string]any{"revision": "5", "payload_sha256": strings.Repeat("b", 64)}
	before := map[string]any{"accepted": maps.Clone(receipt), "applied": map[string]any{"revision": "4", "payload_sha256": strings.Repeat("d", 64)}, "attempt": "10", "phase": "applying"}
	after := map[string]any{"accepted": maps.Clone(receipt), "applied": maps.Clone(receipt), "attempt": "11", "phase": "ready"}
	state := map[string]any{"release_id": strings.Repeat("f", 64), "generation": strings.Repeat("e", 64)}
	proof := map[string]any{"recovered": true, "signal": "SIGKILL", "exit_status": "9", "unit_result": "signal",
		"agent_pid_before": "10", "agent_pid_after": "20", "systemd_control_pid": 30,
		"previous_release": strings.Repeat("a", 64), "next_release": strings.Repeat("f", 64),
		"credentials_before_sha256": strings.Repeat("c", 64), "credentials_after_sha256": strings.Repeat("c", 64),
		"revision": "5", "payload_sha256": strings.Repeat("b", 64), "attempt_before": "10", "attempt_after": "11"}
	return crashEvidence{proof: proof, files: map[string]any{
		"interrupted-agent-before": before, "interrupted-agent-after": after,
		"interrupted-agent-killed": before,
		"interrupted-desired":      map[string]any{"revision": "5", "payload_sha256": strings.Repeat("b", 64), "generation": strings.Repeat("e", 64)},
		"interrupted-final-state":  state,
		"interrupted-transaction": map[string]any{"phase": "pending", "old": map[string]any{"release": "/opt/vpn-nhop/releases/" + strings.Repeat("a", 64)},
			"next": map[string]any{"release": "/opt/vpn-nhop/releases/" + strings.Repeat("f", 64), "state": crashJSON(t, state)}},
	}}
}

func (f crashEvidence) damage(damage string) {
	after := f.files["interrupted-agent-after"].(map[string]any)
	switch damage {
	case "intent":
		after["accepted"].(map[string]any)["payload_sha256"] = strings.Repeat("d", 64)
	case "credentials":
		f.proof["credentials_after_sha256"] = strings.Repeat("d", 64)
	case "signal":
		f.proof["exit_status"] = "0"
	case "phase":
		f.files["interrupted-transaction"].(map[string]any)["phase"] = "committed"
	case "revision":
		f.files["interrupted-desired"].(map[string]any)["revision"] = "6"
	case "attempt":
		after["attempt"], f.proof["attempt_after"] = "10", "10"
	case "final-state":
		f.files["interrupted-final-state"].(map[string]any)["generation"] = strings.Repeat("a", 64)
	case "killed-intent":
		f.files["interrupted-agent-killed"] = after
	}
}

func crashJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func (f crashEvidence) write(t *testing.T, directory string) {
	t.Helper()
	for name, value := range f.files {
		raw := crashJSON(t, value)
		if err := os.WriteFile(filepath.Join(directory, name+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		field := map[string]string{"interrupted-transaction": "journal_sha256", "interrupted-agent-before": "intent_before_sha256", "interrupted-agent-after": "intent_after_sha256"}[name]
		if field != "" {
			f.proof[field] = fmt.Sprintf("%x", sha256.Sum256(raw))
		}
	}
	if err := os.WriteFile(filepath.Join(directory, "fleet.json"), crashJSON(t, map[string]any{"installer_interruption": f.proof}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "interrupted-systemd-killed.txt"), []byte("MainPID=0\nExecMainStatus=9\nResult=signal\n"), 0600); err != nil {
		t.Fatal(err)
	}
}
