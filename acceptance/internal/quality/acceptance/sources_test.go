package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyPreservesTrackedSourcesIgnoredByParentRepository(t *testing.T) {
	f := newFixture(t)
	files := []string{"go.mod", "go.sum", "justfile", "cmd/main.go", "internal/runtime/xray/runtime.go", "internal/control/coverage.go",
		"internal/control/fleet/panel/views/layout.html", "internal/control/fleet/panel/views/style.css", "ops/runtime/xray/source.tsv",
		"examples/nhop/topology.yaml", "examples/nhop/protocols.yaml"}
	for _, name := range files {
		write(t, filepath.Join(f.root, name), "tracked "+name+"\n", 0600)
	}
	for _, args := range [][]string{{"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@invalid", "commit", "-m", "source inputs"}} {
		command := exec.Command(f.realGit, append([]string{"-C", f.root}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("fixture: %v %s", err, output)
		}
	}
	write(t, filepath.Join(f.checkout, ".gitignore"), "xray\ncoverage.go\n", 0600)
	script := filepath.Join(f.root, "ops/testing/acceptance/sources.sh")
	command := exec.Command("bash", "-euo", "pipefail", "-c", `source "$1"; acceptance_copy_sources "$2" "$3" "$4"`, "acceptance-test", script, f.root, f.checkout, f.output)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("source copy: %v %s", err, output)
	}
	command = exec.Command(f.realGit, "-C", f.checkout, "ls-files", "acceptance")
	tracked, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if !strings.Contains(string(tracked), "acceptance/"+name+"\n") {
			t.Fatalf("committed source omitted: %s", name)
		}
	}
}

func TestPublicationRetainsOriginallyEnabledActionsPolicy(t *testing.T) {
	f := newFixture(t)
	command := f.command("publish.sh", f.output)
	command.Env = append(command.Env, "CI_TEST_ENABLED=true")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("fake push failure hidden: %s", output)
	}
	raw, err := os.ReadFile(filepath.Join(f.output, "actions-before.json"))
	if err != nil || !strings.Contains(string(raw), `"enabled":true`) {
		t.Fatalf("original Actions policy lost: %s %v", raw, err)
	}
}
