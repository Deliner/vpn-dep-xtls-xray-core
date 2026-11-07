package linuxlab

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func TestClientLabFailureKeepsOneLogAndFailureStatus(t *testing.T) {
	root := clientFailureFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	command := process.Command(ctx, "bash", "-c", clientFailureDriver, "fixture", root)
	output, err := command.CombinedOutput()
	process.Stop(command)
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 17 || ctx.Err() != nil {
		t.Fatalf("guest failure was hidden or logging did not terminate: %v: %s", err, output)
	}
	runs, err := filepath.Glob(filepath.Join(root, "build/client-kernel/run-*"))
	if err != nil || len(runs) != 1 {
		t.Fatal("missing isolated run", runs, err)
	}
	log, err := os.ReadFile(filepath.Join(runs[0], "guest.log"))
	if err != nil || string(log) != "synthetic guest failure\n" {
		t.Fatalf("guest log duplicated or lost: %d bytes: %v", len(log), err)
	}
	status, err := os.ReadFile(filepath.Join(runs[0], "status"))
	if err != nil || string(status) != "failed\n" || strings.Count(string(output), "synthetic guest failure") != 1 {
		t.Fatalf("failure status/output differs: %s: %s: %v", status, output, err)
	}
}

func clientFailureFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, script := range []string{"testing/client/lab.sh", "testing/client/kernel-guest.sh", "testing/client/resolved-guest.sh", "testing/linux/environment.sh", "runtime/integrity.sh"} {
		raw, err := os.ReadFile("../../../ops/" + script)
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, "ops", script), raw)
	}
	for _, name := range []string{"network", "resolver"} {
		write(t, filepath.Join(root, "internal/control/client", name, "fixture.go"), []byte("package fixture\n"))
	}
	for _, name := range []string{"go.mod", "go.sum", "cmd/nhop-client/main.go"} {
		write(t, filepath.Join(root, name), []byte("fixture input\n"))
	}
	return root
}

const clientFailureDriver = `set -euo pipefail
# Bound damage even if a self-copying failure handler regresses.
ulimit -f 128
root="$1"
source "$root/ops/testing/client/lab.sh"
lab_environment() { lab_root="$root"; lab_data="$root/build"; lab_arch=arm64; lab_lima=true; lab_instance=fixture; }
lab_owned() { :; }
git() { printf 'fixture-source\n'; }
go() {
  if [[ "$1" == version ]]; then printf 'fixture-toolchain\n'; return; fi
  local previous= argument
  for argument in "$@"; do
    if [[ "$previous" == -o ]]; then printf 'fixture-binary\n' > "$argument"; return; fi
    previous="$argument"
  done
  return 2
}
lab_shell() {
  if [[ "$1" == mkdir ]]; then return; fi
  printf 'synthetic guest failure\n'
  return 17
}
client_lab_begin
client_lab_run resolver
`
