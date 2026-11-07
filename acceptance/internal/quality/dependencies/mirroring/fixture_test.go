package mirroring

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/github"
	"vpn-script/internal/quality/dependencies/records"
)

type mirrorFixture struct {
	source                                        Source
	actionsDisabled, pushed, shallow, denyActions bool
	remote                                        string
}

func fixture(t *testing.T) *mirrorFixture {
	t.Helper()
	t.Chdir(t.TempDir())
	source := Source{Name: "vpn-dep-fixture", URL: "https://example.org/source", Commit: strings.Repeat("a", 40)}
	if err := os.MkdirAll("ops/dependencies", 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("ops/dependencies/git-sources.tsv", []byte(source.Name+"\t"+source.URL+"\t"+source.Commit+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	prepared := Snapshot{Source: source, Tree: strings.Repeat("b", 40), Ancestors: 12}
	if err := records.Save("mirror-"+source.Name+".json", prepared); err != nil {
		t.Fatal(err)
	}
	return &mirrorFixture{source: source}
}

func (f *mirrorFixture) run(_ context.Context, binary string, args ...string) ([]byte, error) {
	if binary == "gh" {
		return f.api(args)
	}
	if binary != "env" {
		return nil, fmt.Errorf("unexpected executable %s", binary)
	}
	index := 0
	for index < len(args) && args[index] != "-C" {
		index++
	}
	if index+2 >= len(args) {
		return nil, fmt.Errorf("missing explicit repository")
	}
	command := strings.Join(args[index+2:], " ")
	if strings.Contains(command, "push --porcelain") {
		if !f.actionsDisabled || !strings.Contains(command, "https://github.com/Deliner/vpn-dep-fixture.git "+f.source.Commit+":"+sourceRef(f.source)) {
			return nil, fmt.Errorf("unsafe push before disabling Actions or to a different destination")
		}
		f.pushed, f.remote = true, f.source.Commit+"\t"+sourceRef(f.source)
		return nil, nil
	}
	if strings.HasPrefix(command, "ls-remote ") {
		return []byte(f.remote), nil
	}
	values := map[string]string{"rev-parse --is-bare-repository": "true", "rev-parse --is-shallow-repository": fmt.Sprint(f.shallow),
		"remote get-url upstream": f.source.URL, "rev-parse " + sourceRef(f.source) + "^{commit}": f.source.Commit,
		"rev-parse " + f.source.Commit + "^{tree}": strings.Repeat("b", 40), "rev-list --count " + f.source.Commit: "12", "fsck --full": ""}
	value, found := values[command]
	if !found {
		return nil, fmt.Errorf("unexpected git command: %s", command)
	}
	return []byte(value), nil
}

func (f *mirrorFixture) api(args []string) ([]byte, error) {
	command := strings.Join(args, " ")
	if command == "api --hostname github.com user --jq .login" {
		return []byte("Deliner"), nil
	}
	if command == "api --hostname github.com repos/Deliner/vpn-dep-fixture" {
		return json.Marshal(github.Repository{ID: 42, FullName: "Deliner/vpn-dep-fixture", Description: "Source mirror of " + f.source.URL + "; selected N-hop revisions."})
	}
	if command == "api --hostname github.com --method PUT repos/Deliner/vpn-dep-fixture/actions/permissions -F enabled=false" {
		if f.denyActions {
			return nil, fmt.Errorf("Actions permission denied")
		}
		f.actionsDisabled = true
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected GitHub operation: %s", command)
}

func changePrepared(t *testing.T, source Source) {
	t.Helper()
	if err := safeio.JSON("build/dependencies/mirror-"+source.Name+".json", Snapshot{Source: source, Tree: strings.Repeat("c", 40), Ancestors: 12}); err != nil {
		t.Fatal(err)
	}
}
