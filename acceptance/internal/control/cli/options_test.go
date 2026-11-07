package cli

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestDeployJobsOption(t *testing.T) {
	for _, args := range [][]string{{"bundle"}, {"bundle", "--jobs", "4"}, {"--jobs", "4", "bundle"}} {
		value, err := parse("deploy", args, io.Discard)
		expected := 4
		if len(args) == 1 {
			expected = 1
		}
		if err != nil || value.path != "bundle" || value.jobs != expected {
			t.Fatalf("deploy options %v: %+v %v", args, value, err)
		}
	}
	for _, jobs := range []string{"0", "-1", "65", "invalid"} {
		err := Run(context.Background(), []string{"deploy", "missing", "--jobs", jobs}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "jobs") {
			t.Fatalf("invalid jobs reached bundle access: %s: %v", jobs, err)
		}
	}
}
