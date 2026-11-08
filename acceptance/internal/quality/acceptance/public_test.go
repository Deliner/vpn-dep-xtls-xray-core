package acceptance

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicCollectorRequiresCrashBoundariesAndRetainedKernelIdentity(t *testing.T) {
	for _, damage := range []string{"", "missing-case", "foreign-token", "replaced-handle", "unrecorded", "missing-kernel", "direct-leak", "short-watch", "missing-DNS", "missing-root", "missing-marker", "unexpected-phase"} {
		t.Run(damage, func(t *testing.T) {
			directory := t.TempDir()
			log := damageIntentEvidence(damage, publicEvidence())
			write(t, filepath.Join(directory, "guest.log"), log, 0600)
			command := exec.Command("bash", "-c", `set -euo pipefail; source "$1"; if client_verify_public "$2" tunnel; then exit 0; else exit 1; fi`, "fixture", "../../../ops/testing/acceptance/client/verify.sh", directory)
			output, err := command.CombinedOutput()
			if (err == nil) != (damage == "") {
				t.Fatalf("public evidence acceptance mismatch %s: %v %s", damage, err, output)
			}
		})
	}
}

func damageIntentEvidence(damage, log string) string {
	after := "planned guard resumed: stage=after token=nhop:" + strings.Repeat("b", 32)
	changes := map[string][2]string{
		"missing-case":     {"/intent-after-stop ", "/missing "},
		"foreign-token":    {after, strings.Replace(after, strings.Repeat("b", 32), strings.Repeat("f", 32), 1)},
		"replaced-handle":  {after + " handle=17", after + " handle=18"},
		"unrecorded":       {"handle=0 observed_handle=17", "handle=3 observed_handle=17"},
		"missing-kernel":   {"observed_handle=17", "observed_handle=0"},
		"direct-leak":      {"attempts=200 delivered=0", "attempts=200 delivered=1"},
		"short-watch":      {"attempts=200", "attempts=10"},
		"missing-DNS":      {"system DNS blocked:", "missing DNS blocked:"},
		"missing-root":     {"--- PASS: TestKernelPublicClientAndControllerRecovery (", "--- FAIL: TestKernelPublicClientAndControllerRecovery ("},
		"missing-marker":   {"planned guard recovery verified: stage=after action=resume", "missing verified recovery"},
		"unexpected-phase": {"stage=after", "stage=unknown"},
	}
	if change, ok := changes[damage]; ok {
		return strings.ReplaceAll(log, change[0], change[1])
	}
	return log
}

func publicEvidence() string {
	var log strings.Builder
	log.WriteString("client policy: IPv6=tunnel\n")
	for _, failure := range []string{"graceful", "native", "controller-term", "controller-kill", "reconnect-native", "reconnect-frontend", "resume-term", "resume-kill", "intent-before-resume", "intent-after-resume", "intent-before-stop", "intent-after-stop"} {
		fmt.Fprintf(&log, "--- PASS: TestKernelPublicClientAndControllerRecovery/%s (1.0s)\n", failure)
	}
	for _, action := range []string{"resume", "reconnect"} {
		for range 2 {
			fmt.Fprintf(&log, "%s direct UDP: attempts=9000 delivered=0; same firewall lease retained\n", action)
		}
	}
	for i, fault := range []struct{ stage, action string }{{"before", "resume"}, {"after", "resume"}, {"before", "stop"}, {"after", "stop"}} {
		token, handle := "nhop:"+strings.Repeat(string(rune('a'+i)), 32), 0
		if fault.stage == "after" {
			handle = 17
		}
		fmt.Fprintf(&log, "planned guard crash: stage=%s token=%s handle=0 observed_handle=%d\n", fault.stage, token, handle)
		if fault.action == "resume" {
			fmt.Fprintf(&log, "planned guard resumed: stage=%s token=%s handle=17 attempts=200 delivered=0\n", fault.stage, token)
		}
		fmt.Fprintf(&log, "planned guard recovery verified: stage=%s action=%s\n", fault.stage, fault.action)
	}
	log.WriteString(publicDNSEvidence())
	log.WriteString("--- PASS: TestKernelPublicClientAndControllerRecovery (45.0s)\nPASS\n")
	return log.String()
}

func publicDNSEvidence() string {
	var log strings.Builder
	for index := range 26 {
		fmt.Fprintf(&log, "system DNS direct: direct=%d->%d HTTPS=0->0\n", index, index+1)
	}
	for index := range 18 {
		fmt.Fprintf(&log, "system DNS tunnel: direct=26->26 HTTPS=%d->%d\n", index, index+1)
	}
	for range 13 {
		log.WriteString("system DNS blocked: direct=26->26 HTTPS=18->18\n")
	}
	return log.String()
}
