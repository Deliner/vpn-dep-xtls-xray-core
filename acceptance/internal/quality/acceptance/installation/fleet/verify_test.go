package fleet

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestFleetCollectorRequiresRecoveryAndCompleteConcurrentTraffic(t *testing.T) {
	for _, damage := range []string{"", "recovery", "phase", "streams", "tcp", "udp", "error", "generation", "platform"} {
		t.Run(damage, func(t *testing.T) {
			data := reportFixture()
			damageReport(data, damage)
			raw, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "fleet.json")
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command("bash", "-c", `source "$1"; if fleet_verify_report "$2" amd64; then exit 0; else exit 1; fi`,
				"fixture", "../../../../../ops/testing/acceptance/fleet/verify.sh", path)
			if output, err := command.CombinedOutput(); (err == nil) != (damage == "") {
				t.Fatalf("Fleet evidence mismatch: %v %s", err, output)
			}
		})
	}
}

func reportFixture() map[string]any {
	data := map[string]any{"version": 1, "status": "passed", "platform": "linux/amd64", "revoke_exit_status": "78",
		"generations": []string{strings.Repeat("a", 64), strings.Repeat("b", 64)}}
	for _, flag := range []string{"rollback", "agent_stop_retains_vpn", "controller_recovery", "agent_crash_recovery",
		"credentials_unchanged", "revoke_no_restart", "removed_state_retained"} {
		data[flag] = true
	}
	phases := map[string]any{}
	for _, name := range []string{"initial", "updated", "rolled-back-agent-stopped", "controller-offline", "agent-restarted", "revoked-agent-vpn-retained"} {
		phases[name] = map[string]any{"phase": name, "profiles": 2, "required_rounds": 3, "barrier_rounds": 3,
			"concurrent_tcp_udp_streams": 4, "routes": map[string]any{
				"shadowsocks": map[string]any{"tcp_rounds": 3, "udp_rounds": 3},
				"tuic":        map[string]any{"tcp_rounds": 3, "udp_rounds": 3}}}
	}
	data["traffic"] = phases
	return data
}

func damageReport(data map[string]any, damage string) {
	phases := data["traffic"].(map[string]any)
	phase := phases["initial"].(map[string]any)
	route := phase["routes"].(map[string]any)["tuic"].(map[string]any)
	switch damage {
	case "recovery":
		data["credentials_unchanged"] = false
	case "phase":
		delete(phases, "controller-offline")
	case "streams":
		phase["concurrent_tcp_udp_streams"] = 2
	case "tcp", "udp":
		route[damage+"_rounds"] = 2
	case "error":
		route["error"] = "late response"
	case "generation":
		data["generations"] = []string{strings.Repeat("a", 64), strings.Repeat("a", 64)}
	case "platform":
		data["platform"] = "linux/arm64"
	}
}
