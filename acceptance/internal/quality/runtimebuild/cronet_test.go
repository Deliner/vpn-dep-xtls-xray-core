package runtimebuild

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type cronetFixture struct {
	fixture
	command *exec.Cmd
	library string
}

func newCronetFixture(t *testing.T, platform string) cronetFixture {
	t.Helper()
	f := cronetFixture{fixture: newFixture(t)}
	parts := strings.Split(platform, "/")
	version := "v0.0.0-20261103043047-3aac4ed44c5c"
	module := "github.com/sagernet/cronet-go/lib/" + strings.Join(parts, "_")
	cache := filepath.Join(f.root, "cache")
	directory := filepath.Join(cache, module+"@"+version)
	member := "libcronet.so"
	if parts[0] == "darwin" {
		member = "libcronet.a"
	}
	f.library = filepath.Join(directory, member)
	raw := []byte("verified native library\n")
	write(t, f.library, raw, 0600)
	write(t, filepath.Join(directory, "libcronet_purego.go"), []byte("const Version = \"154.0.8037.49\"\n"), 0600)
	recipe, work := filepath.Join(f.root, "ops/runtime/sing-box"), filepath.Join(f.root, "work")
	pin := fmt.Sprintf("%s\t%s\t%s\t%x\t%d\t154.0.8037.49\n", platform, version, member, sha256.Sum256(raw), len(raw))
	write(t, filepath.Join(recipe, "companions.tsv"), []byte(pin), 0600)
	if err := os.MkdirAll(filepath.Join(work, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(f.root, "tools/go"), []byte(cronetFixtureGo), 0700)
	f.command = exec.Command("bash", "-eu", "-o", "pipefail", "-c", `source "$recipe/build.sh"; candidate_cronet_prepare; candidate_companions`)
	f.command.Env = append(os.Environ(), "root="+f.root, "recipe="+recipe, "work="+work,
		"target_os="+parts[0], "target_arch="+parts[1], "GOMODCACHE="+cache,
		"FIXTURE_NATIVE_DIR="+directory, "PATH="+filepath.Join(f.root, "tools")+":"+os.Getenv("PATH"))
	return f
}

func TestCronetNativeGenerationIsVerifiedBeforePublication(t *testing.T) {
	for _, platform := range []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"} {
		t.Run(platform, func(t *testing.T) {
			f := newCronetFixture(t, platform)
			if output, err := f.command.CombinedOutput(); err != nil {
				t.Fatalf("paired wrapper/library rejected: %v; %s", err, output)
			}
			provenance, err := os.ReadFile(filepath.Join(f.root, "work/companion.txt"))
			if err != nil || !strings.Contains(string(provenance), "companion_chromium_version=154.0.8037.49") ||
				!strings.Contains(string(provenance), "companion_module_sum=h1:verified-fixture") {
				t.Fatalf("native identity missing: %v; %s", err, provenance)
			}
			raw, err := os.ReadFile(f.library)
			if err != nil || string(raw) != "verified native library\n" {
				t.Fatal("module cache changed", err)
			}
			copied, err := os.ReadFile(filepath.Join(f.root, "work/bin/libcronet.so"))
			if strings.HasPrefix(platform, "linux/") {
				if err != nil || string(copied) != string(raw) {
					t.Fatal("verified dynamic library not published", err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatal("static library published as a runtime companion", err)
			}
		})
	}
}

func TestCronetRejectsUnpairedOrChangedNativeInputs(t *testing.T) {
	for _, fault := range []string{"wrapper", "bindings", "native-version", "directory", "checksum", "symlink", "chromium", "cache", "sum", "duplicate"} {
		t.Run(fault, func(t *testing.T) {
			f := newCronetFixture(t, "linux/arm64")
			f.command.Env = append(f.command.Env, "FIXTURE_CRONET_FAULT="+fault)
			switch fault {
			case "checksum":
				write(t, f.library, []byte("unreviewed native data!\n"), 0600)
			case "symlink":
				if err := os.Rename(f.library, f.library+".target"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(f.library+".target", f.library); err != nil {
					t.Fatal(err)
				}
			case "chromium":
				write(t, filepath.Join(filepath.Dir(f.library), "libcronet_purego.go"), []byte("const Version = \"other\"\n"), 0600)
			case "duplicate":
				path := filepath.Join(f.root, "ops/runtime/sing-box/companions.tsv")
				raw, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				write(t, path, append(raw, raw...), 0600)
			}
			if output, err := f.command.CombinedOutput(); err == nil {
				t.Fatalf("unverified native input published: %s", output)
			}
			if _, err := os.Stat(filepath.Join(f.root, "work/bin/libcronet.so")); !os.IsNotExist(err) {
				t.Fatal("failed preflight left a runtime companion", err)
			}
		})
	}
}

const cronetFixtureGo = `#!/usr/bin/env bash
set -eu
[[ "$1" != -C ]] || shift 2
fault="${FIXTURE_CRONET_FAULT:-}"
if [[ "$1 $2" == "mod download" ]]; then exit 0; fi
if [[ "$1 $2" == "mod verify" ]]; then [[ "$fault" != cache ]]; exit; fi
[[ "$1 $2" == "list -m" ]] || exit 42
case "$4" in
  '{{.Version}}')
    case "$5" in
      */lib/*) [[ "$fault" != native-version ]] || { printf 'wrong\n'; exit; }; printf 'v0.0.0-20261103043047-3aac4ed44c5c\n' ;;
      */all) [[ "$fault" != bindings ]] || { printf 'wrong\n'; exit; }; printf 'v0.0.0-20261103043844-095cb9e06046\n' ;;
      *) [[ "$fault" != wrapper ]] || { printf 'wrong\n'; exit; }; printf 'v0.0.0-20261103043844-095cb9e06046\n' ;;
    esac ;;
  '{{.Dir}}') [[ "$fault" != directory ]] || { printf '/unexpected/module\n'; exit; }; printf '%s\n' "$FIXTURE_NATIVE_DIR" ;;
  '{{.Sum}}') [[ "$fault" != sum ]] || exit 0; printf 'h1:verified-fixture\n' ;;
  *) exit 42 ;;
esac
`
