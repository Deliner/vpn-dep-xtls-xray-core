package diagnostics

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestExternalChecksPreserveFailureAndNameSkill(t *testing.T) {
	for _, rule := range []Rule{StaticAnalysis, Formatting, SecurityAudit} {
		command := exec.Command("bash", "../../../ops/quality/check.sh", string(rule), "sh", "-c", "printf 'original diagnostic\\n' >&2; exit 7")
		output, err := command.CombinedOutput()
		var failure *exec.ExitError
		if !errors.As(err, &failure) || failure.ExitCode() != 7 {
			t.Fatalf("exit status changed: %v, %s", err, output)
		}
		if !strings.Contains(string(output), "original diagnostic") || !strings.Contains(string(output), rule.Instruction()) {
			t.Fatalf("external check lost its diagnostic/skill: %s", output)
		}
	}
}

func TestExternalSuccessfulCheckDoesNotPrintRepairSkill(t *testing.T) {
	command := exec.Command("bash", "../../../ops/quality/check.sh", string(Formatting), "sh", "-c", "printf 'success'")
	output, err := command.CombinedOutput()
	if err != nil || string(output) != "success" {
		t.Fatalf("successful check was changed: %v, %s", err, output)
	}
}
