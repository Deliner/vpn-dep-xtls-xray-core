package acceptance

import (
	"os"
	"strings"
	"testing"
)

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
