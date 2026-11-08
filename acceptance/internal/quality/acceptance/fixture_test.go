package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type fixture struct{ root, output, checkout, trace, realGit string }

func write(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{root: root, output: filepath.Join(root, "build/acceptance/test"), trace: filepath.Join(root, "trace"), realGit: realGit}
	f.checkout = filepath.Join(f.output, "repository")
	for _, name := range []string{"identity.sh", "publish.sh", "collect.sh", "run.sh", "sources.sh"} {
		raw, err := os.ReadFile("../../../ops/testing/acceptance/" + name)
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, "ops/testing/acceptance", name), string(raw), 0700)
	}
	write(t, filepath.Join(root, ".gitignore"), "build/\ntools/\ntrace\n", 0600)
	project := gitFixture(t, f, root)
	prepared := gitFixture(t, f, f.checkout)
	for name, content := range map[string]string{"project.commit": project, "prepared.commit": prepared, "repository-name": "Deliner/vpn-dep-xtls-xray-core", "status": "prepared"} {
		write(t, filepath.Join(f.output, name), content+"\n", 0600)
	}
	write(t, filepath.Join(root, "tools/gh"), fakeGitHub, 0700)
	write(t, filepath.Join(root, "tools/git"), fakeGit, 0700)
	return f
}

func gitFixture(t *testing.T, f fixture, directory string) string {
	t.Helper()
	write(t, filepath.Join(directory, "marker"), "owned fixture\n", 0600)
	for _, args := range [][]string{{"init", "--initial-branch=nhop-acceptance"}, {"add", "."}, {"-c", "user.name=Fixture", "-c", "user.email=fixture@invalid", "commit", "-m", "fixture"}, {"remote", "add", "origin", "https://github.com/Deliner/vpn-dep-xtls-xray-core.git"}} {
		command := exec.Command(f.realGit, args...)
		command.Dir = directory
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %v: %s", err, output)
		}
	}
	command := exec.Command(f.realGit, "rev-parse", "HEAD")
	command.Dir = directory
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	return string(output[:len(output)-1])
}

func (f fixture) command(script string, args ...string) *exec.Cmd {
	command := exec.Command("bash", append([]string{filepath.Join(f.root, "ops/testing/acceptance", script)}, args...)...)
	command.Dir = f.root
	command.Env = append(os.Environ(), "CI_TEST_TRACE="+f.trace, "CI_TEST_GIT="+f.realGit, "CI_TEST_OUTPUT="+f.output, "PATH="+filepath.Join(f.root, "tools")+":"+os.Getenv("PATH"))
	return command
}
