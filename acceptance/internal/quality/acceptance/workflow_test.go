package acceptance

import (
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type artifactWorkflow struct {
	Jobs map[string]struct {
		Steps []struct {
			Uses string
			If   string
			With map[string]any
		}
	}
}

func TestProtocolWorkflowPreservesCompleteEvidenceAndCandidates(t *testing.T) {
	raw, err := os.ReadFile("../../../ops/testing/acceptance/workflow.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow artifactWorkflow
	if err := yaml.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{"acceptance/build/acceptance-results": false, "acceptance/build/runtime-candidates": false}
	for _, step := range workflow.Jobs["protocols"].Steps {
		if !strings.HasPrefix(step.Uses, "actions/upload-artifact@") {
			continue
		}
		path, _ := step.With["path"].(string)
		if seen, known := paths[path]; !known || seen {
			t.Fatal("unexpected or duplicate artifact", path)
		}
		if step.If != "always()" || step.With["include-hidden-files"] != true || step.With["if-no-files-found"] != "error" {
			t.Fatal("artifact must retain all evidence, including hidden files, on failure", path)
		}
		paths[path] = true
	}
	for path, found := range paths {
		if !found {
			t.Fatal("required artifact missing", path)
		}
	}
}

func TestRepeatPublicationReenablesOnlyThePreparedWorkflowBeforePush(t *testing.T) {
	f := newFixture(t)
	command := f.command("publish.sh", f.output)
	command.Env = append(command.Env, "CI_TEST_REGISTERED=1")
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("fixture push must still fail: %s", output)
	}
	raw, err := os.ReadFile(f.trace)
	if err != nil {
		t.Fatal(err)
	}
	trace := string(raw)
	disabled := strings.Index(trace, "/workflows/303/disable\n")
	enabled := strings.Index(trace, "/workflows/303/enable\n")
	pushed := strings.Index(trace, "\npush\n")
	if disabled < 0 || enabled <= disabled || pushed <= enabled {
		t.Fatalf("prepared workflow is not runnable before push: %s", trace)
	}
	if strings.Contains(trace, "/101/enable") || strings.Contains(trace, "/102/enable") {
		t.Fatal("publication enabled an inherited workflow")
	}
}
