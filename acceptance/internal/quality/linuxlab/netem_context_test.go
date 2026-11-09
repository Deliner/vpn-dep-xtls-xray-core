package linuxlab

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNetemNativeContextRejectsForeignIdentityAndLocation(t *testing.T) {
	for _, key := range []string{"valid", "GITHUB_ACTIONS", "GITHUB_REPOSITORY", "GITHUB_REPOSITORY_ID", "GITHUB_REPOSITORY_OWNER_ID",
		"RUNNER_ENVIRONMENT", "GITHUB_EVENT_NAME", "GITHUB_REF", "GITHUB_SHA", "GITHUB_RUN_ID", "NHOP_NODE_CI", "GITHUB_WORKSPACE", "TEST_OS"} {
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			root, err := filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			directory := filepath.Join(root, "acceptance/build/masque-acceptance/runs/0-reno/experiment")
			write(t, filepath.Join(directory, "execution-context"), []byte("owned-native-ci\n"))
			script, err := filepath.Abs("../../../ops/testing/diagnostics/netem/context.sh")
			if err != nil {
				t.Fatal(err)
			}
			command := exec.Command("bash", "-c", `source "$1"; uname() { if [[ "$1" == -s ]]; then printf '%s\n' "$TEST_OS"; else printf 'x86_64\n'; fi; }; netem_execution_context`, "context-fixture", script)
			command.Dir, command.Env = directory, nativeContextEnvironment(root)
			if key != "valid" {
				command.Env = append(command.Env, key+"=foreign")
			}
			output, err := command.CombinedOutput()
			if (err == nil) != (key == "valid") {
				t.Fatalf("untrusted context accepted %s: %v %s", key, err, output)
			}
		})
	}
}

func nativeContextEnvironment(root string) []string {
	return append(os.Environ(), "GITHUB_ACTIONS=true", "GITHUB_REPOSITORY=Deliner/vpn-dep-xtls-xray-core", "GITHUB_REPOSITORY_ID=1449369505",
		"GITHUB_REPOSITORY_OWNER_ID=26311467", "RUNNER_ENVIRONMENT=github-hosted", "GITHUB_EVENT_NAME=push", "GITHUB_REF=refs/heads/nhop-acceptance",
		"GITHUB_SHA="+strings.Repeat("a", 40), "GITHUB_RUN_ID=123", "NHOP_NODE_CI=linux/amd64", "GITHUB_WORKSPACE="+root, "TEST_OS=Linux")
}
