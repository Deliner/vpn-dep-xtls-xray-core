package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreSkipsAlreadyInactiveWorkflowAndResumesOtherWorkflows(t *testing.T) {
	f := newFixture(t)
	write(t, filepath.Join(f.output, "actions-before.json"), "{\"enabled\":true,\"allowed_actions\":\"all\"}", 0600)
	write(t, filepath.Join(f.output, "workflows-before.tsv"), "102\tdisabled_fork\t.github/workflows/other.yml\n101\tactive\t.github/workflows/upstream.yml\n", 0600)
	write(t, filepath.Join(f.root, "ops/testing/acceptance/restore-fixture.sh"), restoreFixture, 0700)
	if output, err := f.command("restore-fixture.sh", f.output).CombinedOutput(); err != nil {
		t.Fatalf("restore: %v: %s", err, output)
	}
	trace, err := os.ReadFile(f.trace)
	if err != nil || strings.Contains(string(trace), "/102/disable") || !strings.Contains(string(trace), "/101/enable") {
		t.Fatalf("incorrect workflow restoration: %v: %s", err, trace)
	}
	state, err := os.ReadFile(filepath.Join(f.output, "actions-state"))
	if err != nil || string(state) != "restored\n" {
		t.Fatalf("restoration not committed: %q: %v", state, err)
	}
}

func TestCollectRestoresOnlyCompletedMatchingRunBeforeDownloadFailure(t *testing.T) {
	for _, scenario := range []struct {
		name, setting string
		restored      bool
	}{
		{"download-failed", "CI_TEST_RUN_STATE=completed", true},
		{"run-still-live", "CI_TEST_RUN_STATE=in_progress", false},
		{"different-commit", "CI_TEST_RUN_SHA=" + strings.Repeat("0", 40), false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newFixture(t)
			write(t, filepath.Join(f.output, "actions-before.json"), "{\"enabled\":false}", 0600)
			write(t, filepath.Join(f.output, "workflows-before.tsv"), "101\tactive\t.github/workflows/upstream.yml\n", 0600)
			command := f.command("collect.sh", f.output, "42")
			command.Env = append(command.Env, scenario.setting)
			output, err := command.CombinedOutput()
			if err == nil {
				t.Fatalf("collection unexpectedly succeeded: %s", output)
			}
			assertCollectionRestoration(t, f, scenario.restored, string(output))
		})
	}
}

func assertCollectionRestoration(t *testing.T, f fixture, restored bool, output string) {
	t.Helper()
	state, err := os.ReadFile(filepath.Join(f.output, "actions-state"))
	trace, traceErr := os.ReadFile(f.trace)
	if traceErr != nil {
		t.Fatal(traceErr)
	}
	if restored {
		if err != nil || string(state) != "restored\n" || !strings.Contains(output, "Artifact download unavailable") || !strings.Contains(string(trace), "/101/enable") {
			t.Fatalf("download failure left policy unrestored: state=%q err=%v trace=%s output=%s", state, err, trace, output)
		}
		return
	}
	if !os.IsNotExist(err) || strings.Contains(string(trace), "--method PUT") || strings.Contains(string(trace), "run download") {
		t.Fatalf("unverified run changed policy or downloaded: state=%q err=%v trace=%s", state, err, trace)
	}
}

const restoreFixture = `#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/identity.sh"
acceptance_identity
acceptance_directory "$1"
acceptance_restore_actions
`
