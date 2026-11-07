package modules

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/dependencies/records"
)

func provenanceFixture(t *testing.T) (Report, string) {
	t.Helper()
	t.Chdir(t.TempDir())
	report := Report{Schema: 1, Scopes: Roots()}
	for index := range report.Scopes {
		scope := &report.Scopes[index]
		if err := os.MkdirAll(scope.Directory, 0700); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"go.mod", "go.sum"} {
			if err := os.WriteFile(filepath.Join(scope.Directory, name), []byte(name), 0600); err != nil {
				t.Fatal(err)
			}
		}
		scope.ManifestSHA, scope.SumsSHA = safeio.Hash([]byte("go.mod")), safeio.Hash([]byte("go.sum"))
		scope.Modules = []Module{{Main: true}, {Path: "first", Version: "v1.0.0"}, {Path: "second", Version: "v2.0.0"}}
	}
	if err := records.Save("inventory.json", report); err != nil {
		t.Fatal(err)
	}
	digest, err := records.Read("inventory.json", new(Report))
	if err != nil {
		t.Fatal(err)
	}
	return report, digest
}

func TestMissingOriginsDeduplicatesActualSelectedVersions(t *testing.T) {
	report := Report{Scopes: []Scope{{Modules: []Module{{Main: true},
		{Path: "original", Version: "v1", Replace: &Module{Path: "fork", Version: "v2"}},
		{Path: "fork", Version: "v2"}, {Path: "fork", Version: "v3"},
		{Path: "known", Version: "v1", Origin: &Origin{}},
	}}}}
	requests := missingOrigins(report)
	if len(requests) != 2 || requests[0].Path != "fork" || requests[0].Version != "v2" || requests[1].Version != "v3" {
		t.Fatalf("incorrect requests %+v", requests)
	}
}

func TestDirectOriginPreservesPartialFailureWithoutChangingInventory(t *testing.T) {
	_, inventorySHA := provenanceFixture(t)
	upstreamFailure := errors.New("source unavailable")
	calls := 0
	run := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		calls++
		if calls == 2 {
			return nil, upstreamFailure
		}
		return originResponse(t, "first", "v1.0.0", &Origin{VCS: "git", URL: "https://github.com/o/r", Hash: strings.Repeat("a", 40)}), nil
	}
	batch, err := ResolveMissing(context.Background(), run, 0, 2)
	if !errors.Is(err, upstreamFailure) || len(batch.Results) != 2 || batch.InventorySHA != inventorySHA {
		t.Fatalf("batch=%+v error=%v", batch, err)
	}
	var stored OriginBatch
	if _, err := records.Read("origins-0000.json", &stored); err != nil {
		t.Fatal(err)
	}
	if len(stored.Results) != 2 || stored.Results[0].Origin == nil || stored.Results[1].Failure != upstreamFailure.Error() {
		t.Fatalf("checkpoint lost evidence: %+v", stored)
	}
	if after, err := records.Read("inventory.json", new(Report)); err != nil || after != inventorySHA {
		t.Fatalf("inventory changed: %s, %v", after, err)
	}
}

func TestDirectOriginRefusesStaleInventoryBeforeQuery(t *testing.T) {
	provenanceFixture(t)
	if err := os.WriteFile("ops/runtime/xray/go.mod", []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		t.Fatal("queried source despite changed module locks")
		return nil, nil
	}
	if _, err := ResolveMissing(context.Background(), run, 0, 1); err == nil {
		t.Fatal("accepted stale inventory")
	}
}
