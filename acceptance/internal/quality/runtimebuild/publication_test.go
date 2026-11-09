package runtimebuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPublicationRestoresActionsOnSuccessAndFailure(t *testing.T) {
	for _, scenario := range []string{"success", "registration", "publish-failure", "live-run", "policy", "identity"} {
		t.Run(scenario, func(t *testing.T) {
			root, original := publicationFixture(t, scenario)
			command := exec.Command("bash", "ops/release/runtime/singbox/actions.sh", "stage")
			command.Dir = root
			command.Env = append(os.Environ(), "PATH="+filepath.Join(root, "tools")+":"+os.Getenv("PATH"), "FIXTURE_ROOT="+root, "FIXTURE_SCENARIO="+scenario)
			output, err := command.CombinedOutput()
			if (err == nil) != (scenario == "success" || scenario == "registration") {
				t.Fatalf("scenario %s: %v, %s", scenario, err, output)
			}
			after, err := os.ReadFile(filepath.Join(root, "policy.json"))
			if err != nil || string(after) != original {
				t.Fatalf("original Actions policy was not preserved: %s, %v", after, err)
			}
			_, published := os.Stat(filepath.Join(root, "publication-attempted"))
			want := scenario == "success" || scenario == "registration" || scenario == "publish-failure"
			if (published == nil) != want {
				t.Fatal("unexpected publication attempt", scenario, published)
			}
		})
	}
}

func publicationFixture(t *testing.T, scenario string) (string, string) {
	t.Helper()
	root := t.TempDir()
	raw, err := os.ReadFile("../../../ops/release/runtime/singbox/actions.sh")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "ops/release/runtime/singbox/actions.sh"), raw, 0600)
	write(t, filepath.Join(root, "ops/release/runtime/singbox/verify.sh"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0600)
	write(t, filepath.Join(root, "ops/release/runtime/singbox/publish.sh"), []byte(publicationFixturePublisher), 0600)
	write(t, filepath.Join(root, "tools/gh"), []byte(publicationFixtureGH), 0700)
	policy := "{\"enabled\":true,\"allowed_actions\":\"all\",\"sha_pinning_required\":false}\n"
	write(t, filepath.Join(root, "policy.json"), []byte(policy), 0600)
	if err := os.MkdirAll(filepath.Join(root, "build/runtime-releases"), 0700); err != nil {
		t.Fatal(err)
	}
	return root, policy
}

const publicationFixturePublisher = `#!/usr/bin/env bash
set -eu
[[ "$(cat "$FIXTURE_ROOT/policy.json")" == '{"enabled":false,"sha_pinning_required":false}' ]] || exit 98
printf 'attempted\n' > "$FIXTURE_ROOT/publication-attempted"
[[ "$FIXTURE_SCENARIO" != publish-failure ]] || exit 17
`

const publicationFixtureGH = `#!/usr/bin/env bash
set -eu
case "$*" in
  *'/actions/permissions'*)
    if [[ "$*" == *'--method PUT'* ]]; then
      while (($#)); do
        if [[ "$1" == --input ]]; then cp "$2" "$FIXTURE_ROOT/policy.json"; exit 0; fi
        if [[ "$1" == enabled=false ]]; then printf '{"enabled":false,"sha_pinning_required":false}\n' > "$FIXTURE_ROOT/policy.json"; exit 0; fi
        shift
      done
      exit 97
    fi
    if [[ "$*" == *'--jq'* ]]; then
      if [[ "$FIXTURE_SCENARIO" == policy ]]; then printf 'true\tselected\tfalse\n'; else printf 'true\tall\tfalse\n'; fi
    else cat "$FIXTURE_ROOT/policy.json"; fi ;;
  *'/actions/runs?'*) [[ "$FIXTURE_SCENARIO" != live-run ]] || printf '42\tin_progress\n' ;;
  *'/actions/workflows?'*)
    printf '123\tactive\t.github/workflows/release.yml\n'
    if [[ "$FIXTURE_SCENARIO" == registration && -f "$FIXTURE_ROOT/publication-attempted" ]]; then printf '456\tactive\t.github/workflows/build.yml\n'; fi ;;
  *' user '*)
    if [[ "$FIXTURE_SCENARIO" == identity ]]; then printf 'Other\t1\n'; else printf 'Deliner\t26311467\n'; fi ;;
  *'repos/Deliner/vpn-dep-sagernet-sing-box'*) printf '1449371112\t26311467\tDeliner/vpn-dep-sagernet-sing-box\tfalse\ttrue\n' ;;
  *) exit 96 ;;
esac
`
