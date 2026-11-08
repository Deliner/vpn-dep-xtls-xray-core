package native

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectionRequiresActualLinkInputsAndCompilerDependencies(t *testing.T) {
	for _, damage := range []string{"", "missing-response", "symlink-response", "missing-headers", "stale-headers", "query-failure"} {
		t.Run(damage, func(t *testing.T) {
			root := inspectionFixture(t, damage)
			command := exec.Command("bash", "-c", inspectionInvocation, "fixture", "../../../../ops/runtime/cronet/inspection.sh", root)
			output, err := command.CombinedOutput()
			if (err == nil) != (damage == "") {
				t.Fatalf("inspection acceptance mismatch: %v %s", err, output)
			}
			if damage == "" {
				checkResponseArchive(t, root)
				for name, want := range map[string]string{"link-target.txt": "libcronet.so\n", "link-response.txt": "obj/library.o -lfixture\n", "header-dependencies.txt": "obj/library.o: #deps 2, deps mtime 1 (VALID)\n    library.cc\n    library.h\n"} {
					data, err := os.ReadFile(filepath.Join(root, "inspection", name))
					if err != nil || string(data) != want {
						t.Fatalf("lost native evidence %s: %v %q", name, err, data)
					}
				}
			}
		})
	}
}

func inspectionFixture(t *testing.T, damage string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"src/gn/out/gn": "#!/bin/bash\nprintf '//components/cronet:cronet\n'\n",
		"src/components/cronet/native/include/cronet_c.h": "native ABI header\n",
		"out/args.gn": "target_cpu=\"arm64\"\n", "out/libcronet.so.rsp": "obj/library.o -lfixture\n",
		"out/obj/fixture.a.rsp": "obj/archive-member.o\n",
		"tools/ninja-real":      inspectionNinja, "damage": damage,
	}
	for name, data := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0700); err != nil {
			t.Fatal(err)
		}
	}
	response := filepath.Join(root, "out/libcronet.so.rsp")
	if strings.Contains(damage, "response") {
		if err := os.Remove(response); err != nil {
			t.Fatal(err)
		}
	}
	if damage == "symlink-response" {
		if err := os.Symlink(filepath.Join(root, "out/args.gn"), response); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func checkResponseArchive(t *testing.T, root string) {
	t.Helper()
	archive := filepath.Join(root, "inspection/response-files.tar")
	output, err := exec.Command("tar", "-tf", archive).CombinedOutput()
	if err != nil || string(output) != "./libcronet.so.rsp\n./obj/fixture.a.rsp\n" {
		t.Fatalf("incomplete linker/archiver evidence: %v %s", err, output)
	}
	output, err = exec.Command("tar", "-xOf", archive, "./obj/fixture.a.rsp").CombinedOutput()
	if err != nil || string(output) != "obj/archive-member.o\n" {
		t.Fatalf("changed archiver response bytes: %v %s", err, output)
	}
}

const inspectionInvocation = `set -euo pipefail
source "$1"
work="$2"
src="$work/src"
native_output="$work/out"
native_os=linux
if native_inspect; then exit 0; else exit 1; fi
`

const inspectionNinja = `#!/bin/bash
set -euo pipefail
[[ "$1" == -C && "$3" == -t ]]
damage="$(cat "$(dirname "$2")/damage")"
case "$4" in
  commands) printf 'clang++ -shared -o libcronet.so @libcronet.so.rsp\n' ;;
  deps)
    [[ "$damage" != missing-headers ]] || exit 0
    state=VALID
    [[ "$damage" != stale-headers ]] || state=STALE
    printf 'obj/library.o: #deps 2, deps mtime 1 (%s)\n    library.cc\n    library.h\n' "$state"
    ;;
  query) [[ "$damage" != query-failure ]] || exit 3; printf 'libcronet.so:\n  input: solink\n    obj/library.o\n' ;;
  *) exit 2 ;;
esac
`
