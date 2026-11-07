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

const restoreFixture = `#!/usr/bin/env bash
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/identity.sh"
acceptance_identity
acceptance_directory "$1"
acceptance_restore_actions
`
