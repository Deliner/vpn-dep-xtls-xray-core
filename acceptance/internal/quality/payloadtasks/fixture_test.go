package payloadtasks

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

type fixture struct {
	root, trace, report string
	binaries            []string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{root: root, trace: filepath.Join(root, "trace"), report: filepath.Join(root, "build/report")}
	for _, script := range []string{"payload.sh", "compatibility.sh"} {
		data, err := os.ReadFile("../../../ops/testing/" + script)
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, "ops/testing", script), data, 0600)
	}
	for _, name := range []string{"build/xray-test/xray", "build/sing-box-test/sing-box", "bin/candidate xray", "bin/candidate singbox"} {
		path := filepath.Join(root, name)
		write(t, path, []byte(name), 0700)
		f.binaries = append(f.binaries, path)
	}
	write(t, filepath.Join(root, "tools/go"), []byte(fakeGo), 0700)
	write(t, filepath.Join(root, "tools/just"), []byte(fakeJust), 0700)
	return f
}

func write(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) command(script string, args ...string) *exec.Cmd {
	command := exec.Command("bash", append([]string{filepath.Join(f.root, "ops/testing", script)}, args...)...)
	command.Dir = f.root
	command.Env = append(os.Environ(), "TASK_TRACE="+f.trace, "PATH="+filepath.Join(f.root, "tools")+":"+os.Getenv("PATH"))
	return command
}

const fakeGo = `#!/usr/bin/env bash
if [[ "${!#}" == ./internal/control/probe ]]; then
  printf '%s\n' "$XRAY_BINARY" "$SING_BOX_BINARY" "$CGO_ENABLED" > "$TASK_TRACE.probe"
  printf '%s\n' "$@" > "$TASK_TRACE.probe-args"
  if [[ "${FAIL_PROBE:-}" == 1 ]]; then exit 43; fi
  exit 0
fi
printf '%s\n' "$XRAY_BINARY" "$SINGBOX_BINARY" "$XRAY_CLIENT_BINARY" "$SINGBOX_CLIENT_BINARY" "$XRAY_UPDATED_BINARY" "$SINGBOX_UPDATED_BINARY" "$NHOP_UPDATED_NODES" > "$TASK_TRACE"
`

const fakeJust = `#!/usr/bin/env bash
case "$1" in
  payload-runtimes) stage="$(basename "$7")" ;;
  payload-report) stage="$(basename "$2")" ;;
  *) exit 2 ;;
esac
printf '%s %s\n' "$1" "$stage" >> "$TASK_TRACE"
if [[ "$1" == "$FAIL_ACTION" && "$stage" == "$FAIL_STAGE" ]]; then exit 42; fi
exit 0
`
