package runtimerelease

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublisherOnlyPublishesAfterVerifiedDraftDownload(t *testing.T) {
	for _, fault := range []string{"", "existing", "input", "owner", "actions", "commit", "readback", "identity", "create"} {
		t.Run(fault, func(t *testing.T) {
			root, command := publisherFixture(t, fault)
			output, err := command.CombinedOutput()
			_, published := os.Stat(filepath.Join(root, "remote/published"))
			if fault == "" || fault == "existing" {
				if err != nil || published != nil {
					t.Fatalf("verified publication failed: %v; %s", err, output)
				}
			} else if err == nil || !os.IsNotExist(published) {
				t.Fatalf("failed preflight/readback published a release: %v; %s", err, output)
			}
			created, _ := os.ReadFile(filepath.Join(root, "created"))
			if (fault == "input" || fault == "owner" || fault == "actions" || fault == "commit" || fault == "existing") && len(created) != 0 {
				t.Fatal("release created before identity checks or recreated on retry")
			}
		})
	}
}

func publisherFixture(t *testing.T, fault string) (string, *exec.Cmd) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(root, "build/runtime-releases/stage")
	for _, name := range []string{"Xray-linux-amd64.zip", "Xray-linux-arm64.zip", "xray-amd64.json", "xray-arm64.json", "SHA256SUMS", "notes.md"} {
		write(t, filepath.Join(stage, name), []byte("reviewed fixture"))
	}
	write(t, filepath.Join(stage, "tag"), []byte("nhop-xray-26.10.31-fixture\n"))
	write(t, filepath.Join(stage, "target"), []byte(strings.Repeat("a", 40)+"\n"))
	path := filepath.Join(root, "ops/release/runtime/publish.sh")
	write(t, path, read(t, "../../../ops/release/runtime/publish.sh"))
	for name, script := range map[string]string{"go": publisherGo, "gh": publisherGH} {
		path := filepath.Join(root, "tools", name)
		write(t, path, []byte(script))
		if err := os.Chmod(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.Command("bash", path, stage)
	command.Env = append(os.Environ(), "PATH="+filepath.Join(root, "tools")+":"+os.Getenv("PATH"),
		"PUBLISH_FIXTURE_ROOT="+root, "PUBLISH_FIXTURE_STAGE="+stage, "PUBLISH_FIXTURE_FAULT="+fault)
	return root, command
}

const publisherGo = `#!/usr/bin/env bash
set -eu
[[ "$1 $2 $3 $4" == 'run ./cmd/runtimepack verify ops/release/runtime/xray.json' ]]
[[ "$PUBLISH_FIXTURE_FAULT" != input ]] || exit 42
cmp "$PUBLISH_FIXTURE_STAGE/Xray-linux-arm64.zip" "$5/Xray-linux-arm64.zip"
`

const publisherGH = `#!/usr/bin/env bash
set -eu
root="$PUBLISH_FIXTURE_ROOT"
stage="$PUBLISH_FIXTURE_STAGE"
fault="$PUBLISH_FIXTURE_FAULT"
tag="$(cat "$stage/tag")"
target="$(cat "$stage/target")"
case "$*" in
  'api --hostname github.com user '*) printf 'Deliner\t26311467\n' ;;
  'api --hostname github.com repos/Deliner/vpn-dep-xtls-xray-core --jq '*)
    [[ "$fault" != owner ]] || { printf 'another-owner\n'; exit; }
    printf '1449369505\t26311467\tDeliner/vpn-dep-xtls-xray-core\tfalse\ttrue\n' ;;
  *'/actions/permissions --jq .enabled') [[ "$fault" != actions ]] || { printf 'true\n'; exit; }; printf 'false\n' ;;
  *'/commits/'*' --jq .sha') [[ "$fault" != commit ]] || exit 42; printf '%s\n' "$target" ;;
  *'/releases?per_page=100 '* )
    if [[ "$fault" == existing ]]; then mkdir -p "$root/remote"; cp "$stage/"* "$root/remote/"; printf '%s\t123\n' "$tag"; fi ;;
  'release create '*)
    [[ "$fault" != create ]] || exit 42
    mkdir -p "$root/remote"; cp "$stage/"* "$root/remote/"; printf 'created\n' > "$root/created" ;;
  'release download '*)
    destination="${@: -1}"; cp "$root/remote/"* "$destination/"
    if [[ "$fault" == readback ]]; then printf 'changed\n' > "$destination/Xray-linux-arm64.zip"; fi ;;
  'release edit '*) touch "$root/remote/published" ;;
  *'/git/ref/tags/'*' --jq .object.sha') printf '%s\n' "$target" ;;
  'release view '* )
    case "$*" in
      *'--jq .isDraft') if [[ -f "$root/remote/published" ]]; then printf 'false\n'; else printf 'true\n'; fi ;;
      *'--jq '*) [[ "$fault" != identity ]] || exit 42; printf '%s\t%s\ttrue\n' "$tag" "$target" ;;
      *) printf '{}\n' ;;
    esac ;;
  *) printf 'Unexpected gh operation\n' >&2; exit 42 ;;
esac
`
