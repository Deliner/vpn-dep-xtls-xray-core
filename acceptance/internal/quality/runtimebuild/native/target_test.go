package native

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func targetFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"src/chrome/build/linux.pgo.txt": "fixture.profdata",
		"recipe/inputs.tsv":              "pgo-linux\tdigest\thttps://example.invalid/fixture.profdata\n",
		"inputs/pgo-linux":               "profile",
		"sysroot/lib/marker":             "reviewed sysroot",
	}
	for path, content := range files {
		name := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, arch := range []string{"amd64", "arm64"} {
		command := exec.Command("tar", "-cf", filepath.Join(root, "inputs/sysroot-linux-"+arch), "-C", filepath.Join(root, "sysroot"), "lib")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("fixture sysroot: %v %s", err, output)
		}
	}
	return root
}

func TestLinuxCrossTargetPreparesBothReviewedHostAndTargetSysroots(t *testing.T) {
	root := targetFixture(t)
	script, err := filepath.Abs("../../../../ops/runtime/cronet/toolchain.sh")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", "-euc", targetPreparation, "fixture", script, root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("cross target preparation: %v %s", err, output)
	}
	raw, err := os.ReadFile(filepath.Join(root, "requested"))
	if err != nil || string(raw) != "pgo-linux\nsysroot-linux-amd64\nsysroot-linux-arm64\n" {
		t.Fatalf("unreviewed or missing input: %s %v", raw, err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		marker := filepath.Join(root, "src/out/sysroot-build/bullseye/bullseye_"+arch+"_staging/lib/marker")
		if raw, err := os.ReadFile(marker); err != nil || !strings.Contains(string(raw), "reviewed") {
			t.Fatalf("missing %s sysroot: %s %v", arch, raw, err)
		}
	}
}

const targetPreparation = `source "$1"
work="$2"
recipe="$2/recipe"
src="$2/src"
target=linux/arm64
native_host=linux-amd64
native_pin_input() {
  printf '%s\n' "$1" >> "$work/requested"
  printf '%s/inputs/%s\n' "$work" "$1"
}
native_prepare_target
`
