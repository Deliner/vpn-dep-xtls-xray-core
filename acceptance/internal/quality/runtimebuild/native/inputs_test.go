package native

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func inputFixture(t *testing.T, digest string) (string, *exec.Cmd) {
	t.Helper()
	root := t.TempDir()
	script, err := os.ReadFile("../../../../ops/runtime/cronet/review-input.sh")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"ops/runtime/cronet/review-input.sh": script,
		"ops/runtime/cronet/inputs.tsv":      []byte("ninja-linux\t" + digest + "\thttps://example.invalid/ninja.zip\n"),
		"tools/curl":                         []byte(fakeDownload),
	}
	for path, body := range files {
		name := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, body, 0700); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("bash", filepath.Join(root, "ops/runtime/cronet/review-input.sh"), "ninja-linux")
	command.Env = append(os.Environ(), "PATH="+filepath.Join(root, "tools")+":"+os.Getenv("PATH"), "INPUT_REVIEW_EXECUTED="+filepath.Join(root, "executed"))
	return root, command
}

func TestNativeInputReviewWorksWithNounsetAndNeverExecutesDownload(t *testing.T) {
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte(downloadedExecutable)))
	root, command := inputFixture(t, digest)
	output, err := command.CombinedOutput()
	if err != nil || !strings.Contains(string(output), digest) {
		t.Fatalf("native input review failed: %v %s", err, output)
	}
	if _, err := os.Stat(filepath.Join(root, "executed")); !os.IsNotExist(err) {
		t.Fatal("review executed downloaded input")
	}
}

func TestNativeInputReviewRejectsDifferentBytesAndRetainsEvidence(t *testing.T) {
	root, command := inputFixture(t, strings.Repeat("0", 64))
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "differs from its reviewed checksum") {
		t.Fatalf("changed native input accepted: %v %s", err, output)
	}
	paths, err := filepath.Glob(filepath.Join(root, "build/native-inputs/ninja-linux/review-*/observed.tsv"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("failed input evidence missing: %v %v", paths, err)
	}
}

const downloadedExecutable = "#!/bin/sh\nprintf executed > \"$INPUT_REVIEW_EXECUTED\"\n"

const fakeDownload = `#!/bin/sh
while [ "$#" -gt 0 ]; do
  case "$1" in --output) output="$2"; shift ;; --dump-header) headers="$2"; shift ;; esac
  shift
done
printf 'HTTP/2 200\n' > "$headers"
printf '#!/bin/sh\nprintf executed > "$INPUT_REVIEW_EXECUTED"\n' > "$output"
`
