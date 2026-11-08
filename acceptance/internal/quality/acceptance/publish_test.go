package acceptance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicationPreservesOmittedDisabledPolicyAndPinning(t *testing.T) {
	f := newFixture(t)
	command := f.command("publish.sh", f.output)
	command.Env = append(command.Env, "CI_TEST_POLICY=absent", "CI_TEST_PINNING=true")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("fixture push must remain a failure: %s", output)
	}
	raw, err := os.ReadFile(filepath.Join(f.output, "actions-before.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy map[string]any
	if err := json.Unmarshal(raw, &policy); err != nil || len(policy) != 2 || policy["enabled"] != false || policy["sha_pinning_required"] != true {
		t.Fatalf("invented or lost the original policy: %s %v", raw, err)
	}
	trace, err := os.ReadFile(f.trace)
	if err != nil || !strings.HasSuffix(string(trace), "push\n") || strings.Count(string(trace), "/actions/permissions --jq") != 2 {
		t.Fatalf("enabled policy was not verified before push: %s %v", trace, err)
	}
}

func TestPublicationRestoresUnreviewedOrUnreadableEnabledPolicy(t *testing.T) {
	for _, scenario := range []string{"CI_TEST_ACTIVE_POLICY=selected", "CI_TEST_POLICY_READ_FAILURE=1"} {
		f := newFixture(t)
		command := f.command("publish.sh", f.output)
		command.Env = append(command.Env, "CI_TEST_POLICY=absent", scenario)
		if output, err := command.CombinedOutput(); err == nil {
			t.Fatalf("unreviewed policy accepted: %s", output)
		}
		trace, err := os.ReadFile(f.trace)
		if err != nil || strings.Contains(string(trace), "\npush\n") || !strings.Contains(string(trace), "/actions/permissions --input") {
			t.Fatalf("unreviewed policy was not restored without publishing: %s %v", trace, err)
		}
		state, err := os.ReadFile(filepath.Join(f.output, "actions-state"))
		if err != nil || string(state) != "restored\n" {
			t.Fatalf("missing restoration evidence: %q %v", state, err)
		}

	}
}

func TestPublicationCannotEnableActionsBeforeInheritedWorkflowsAreDisabled(t *testing.T) {
	f := newFixture(t)
	command := f.command("publish.sh", f.output)
	command.Env = append(command.Env, "CI_TEST_DISABLE_FAILURE=1")
	output, err := command.CombinedOutput()
	trace, readErr := os.ReadFile(f.trace)
	if err == nil || readErr != nil || !strings.Contains(string(trace), "/101/disable") {
		t.Fatalf("expected inherited workflow refusal: %v %s %s", err, trace, output)
	}
	if strings.Contains(string(trace), "enabled=true") || strings.Contains(string(trace), "\npush\n") {
		t.Fatal("unsafe enable or push after failure")
	}
}

func TestPublicationRefusesUnreviewedActionsPolicy(t *testing.T) {
	f := newFixture(t)
	command := f.command("publish.sh", f.output)
	command.Env = append(command.Env, "CI_TEST_POLICY=selected")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("existing Actions policy changed: %s", output)
	}
	trace, err := os.ReadFile(f.trace)
	if err != nil || strings.Contains(string(trace), "--method PUT") {
		t.Fatalf("Actions mutation was not refused: %s %v", trace, err)
	}
}

func TestPublicationPreservesUncertainPushAsFailure(t *testing.T) {
	f := newFixture(t)
	if output, err := f.command("publish.sh", f.output).CombinedOutput(); err == nil {
		t.Fatalf("failed push accepted: %s", output)
	}
	trace, err := os.ReadFile(f.trace)
	if err != nil || !strings.HasSuffix(string(trace), "push\n") {
		t.Fatalf("verified preparation did not reach test push: %s %v", trace, err)
	}
	status, err := os.ReadFile(filepath.Join(f.output, "status"))
	if err != nil || string(status) != "push-in-progress\n" {
		t.Fatalf("uncertain remote state was lost: %q %v", status, err)
	}
}

func TestPublicationRejectsChangedPreparedCheckout(t *testing.T) {
	f := newFixture(t)
	write(t, filepath.Join(f.checkout, "marker"), "changed", 0600)
	if output, err := f.command("publish.sh", f.output).CombinedOutput(); err == nil {
		t.Fatalf("dirty checkout published: %s", output)
	}
	trace, err := os.ReadFile(f.trace)
	if err != nil || strings.Contains(string(trace), "--method PUT") {
		t.Fatalf("dirty checkout mutated GitHub: %s %v", trace, err)
	}
}

func TestAcceptanceRejectsDifferentNativeCPU(t *testing.T) {
	f := newFixture(t)
	write(t, filepath.Join(f.root, "tools/uname"), "#!/bin/sh\nif [ \"$1\" = -s ]; then echo Linux; else echo x86_64; fi\n", 0700)
	output, err := f.command("run.sh", "arm64").CombinedOutput()
	if err == nil || !strings.Contains(string(output), "CPU does not match") {
		t.Fatalf("foreign architecture accepted: %v %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(f.root, "build/acceptance-results")); !os.IsNotExist(err) {
		t.Fatal("wrong CPU produced acceptance results")
	}
}
