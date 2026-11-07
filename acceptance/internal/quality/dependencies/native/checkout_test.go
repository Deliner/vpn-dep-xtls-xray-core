package native

import (
	"context"
	"strings"
	"testing"
)

func TestInventoryRejectsChangedOrWrongSourceBeforeReadingFiles(t *testing.T) {
	revision := strings.Repeat("a", 40)
	for _, status := range []struct{ head, changes string }{{strings.Repeat("b", 40), ""}, {revision, " M source.cc\n"}, {revision, "?? injected/README.chromium\n"}} {
		run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
			switch args[2] {
			case "rev-parse":
				return []byte(status.head), nil
			case "status":
				return []byte(status.changes), nil
			default:
				t.Fatalf("read changed source with %q", args)
				return nil, nil
			}
		}
		if _, err := Inventory(context.Background(), run, "unused", revision); err == nil {
			t.Fatal("accepted changed source")
		}
	}
}
