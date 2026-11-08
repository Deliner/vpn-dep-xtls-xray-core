package acceptance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/core/protocol"
)

func TestProtocolCollectorRejectsIncompleteOrContradictoryEvidence(t *testing.T) {
	for _, damage := range []string{"", "missing-route", "duplicate-route", "wrong-path", "wrong-engine", "unknown-profile", "missing-pass",
		"short-dns", "dns-leak", "empty-canary", "udp-leak", "wrong-policy", "failed", "missing-root"} {
		t.Run(damage, func(t *testing.T) {
			directory := t.TempDir()
			log := damagedProtocolEvidence(protocolEvidence(), damage)
			write(t, filepath.Join(directory, "guest.log"), log, 0600)
			data, err := protocolCollector(directory, "tunnel").CombinedOutput()
			if (err == nil) != (damage == "") {
				t.Fatalf("protocol evidence acceptance mismatch: %s: %v %s", damage, err, data)
			}
		})
	}
}

func protocolCollector(directory, policy string) *exec.Cmd {
	// Conditional invocation also checks that verifiers propagate errors when
	// Bash errexit is suppressed by their caller.
	script := `set -euo pipefail; source "$1"; source "$2"; if client_verify_protocols "$3" "$4"; then exit 0; else exit 1; fi`
	return exec.Command("bash", "-c", script, "fixture", "../../../ops/testing/acceptance/client/verify.sh",
		"../../../ops/testing/acceptance/client/verify-protocols.sh", directory, policy)
}

func protocolEvidence() string {
	var specs []protocol.Spec
	for _, spec := range protocol.All() {
		if spec.Status == "implemented" {
			specs = append(specs, spec)
		}
	}
	var output strings.Builder
	output.WriteString("TUN matrix policy: IPv6=tunnel; real CLI import/start/reconnect/stop\n")
	for _, hops := range []int{1, 3} {
		for index := range specs {
			output.WriteString(protocolCaseEvidence(specs, index, hops))
		}
	}
	output.WriteString("--- PASS: TestKernelAllProtocolTUN (120.00s)\nPASS\n")
	return output.String()
}

func protocolCaseEvidence(specs []protocol.Spec, index, hops int) string {
	var path []string
	for hop := range hops {
		path = append(path, fmt.Sprintf("node%d.%s", hop, specs[(index+hop)%len(specs)].ID))
	}
	spec := specs[index]
	direct, https := 2*(index+(hops-1)/2*len(specs)), 4*(index+(hops-1)/2*len(specs))
	var output strings.Builder
	fmt.Fprintf(&output, "TUN route=%s engine=%s path=[%s]\n", spec.ID, spec.Engine, strings.Join(path, " "))
	fmt.Fprintf(&output, "system DNS direct: direct=%d->%d HTTPS=%d->%d\n", direct, direct+1, https, https)
	fmt.Fprintf(&output, "system DNS tunnel: direct=%d->%d HTTPS=%d->%d\n", direct+1, direct+1, https, https+2)
	fmt.Fprintf(&output, "system DNS blocked: direct=%d->%d HTTPS=%d->%d\n", direct+1, direct+1, https+2, https+2)
	fmt.Fprintf(&output, "system DNS tunnel: direct=%d->%d HTTPS=%d->%d\n", direct+1, direct+1, https+2, https+4)
	fmt.Fprintf(&output, "system DNS direct: direct=%d->%d HTTPS=%d->%d\n", direct+1, direct+2, https+4, https+4)
	fmt.Fprintf(&output, "TUN protocol=%s hops=%d IPv6=tunnel direct UDP: attempts=1500 delivered=0; same firewall lease retained\n", spec.ID, hops)
	fmt.Fprintf(&output, "--- PASS: TestKernelAllProtocolTUN/%d-hop/%s (2.50s)\n", hops, spec.ID)
	return output.String()
}

func damagedProtocolEvidence(log, damage string) string {
	replacements := map[string][2]string{
		"missing-route":   {"TUN route=masque-h3", "missing route=masque-h3"},
		"wrong-path":      {"node2.masque-h3", "node2.masque-h2"},
		"wrong-engine":    {"engine=xray", "engine=unknown"},
		"unknown-profile": {"masque-h2", "unknown-protocol"},
		"missing-pass":    {"--- PASS: TestKernelAllProtocolTUN/3-hop/masque-h2", "missing pass"},
		"short-dns":       {"system DNS blocked:", "missing DNS blocked:"},
		"dns-leak":        {"direct=1->1 HTTPS=0->2", "direct=1->2 HTTPS=0->2"},
		"empty-canary":    {"attempts=1500", "attempts=0"},
		"udp-leak":        {"delivered=0;", "delivered=1;"},
		"wrong-policy":    {"hops=3 IPv6=tunnel", "hops=3 IPv6=block"},
		"missing-root":    {"--- PASS: TestKernelAllProtocolTUN (", "missing root ("},
	}
	if pair, found := replacements[damage]; found {
		return strings.Replace(log, pair[0], pair[1], 1)
	}
	if damage == "duplicate-route" {
		return log + "TUN route=anytls engine=sing-box path=[node0.anytls]\n"
	}
	if damage == "failed" {
		return log + "--- FAIL: TestKernelAllProtocolTUN (120.00s)\nFAIL\n"
	}
	return log
}

func TestProtocolCollectorRecordedEvidence(t *testing.T) {
	root := os.Getenv("NHOP_CLIENT_PROTOCOL_EVIDENCE")
	if root == "" {
		t.Skip("optional completed, independently verified Linux fixture")
	}
	data, err := os.ReadFile(filepath.Join(root, "guest.log"))
	if err != nil {
		t.Fatal(err)
	}
	policy := "tunnel"
	if strings.Contains(string(data), "TUN matrix policy: IPv6=block;") {
		policy = "block"
	}
	output, err := protocolCollector(root, policy).CombinedOutput()
	if err != nil {
		t.Fatalf("recorded protocol evidence rejected: %v %s", err, output)
	}
}
