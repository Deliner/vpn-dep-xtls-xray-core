package modules

import (
	"strings"
	"testing"

	"vpn-script/internal/quality/dependencies/records"
)

func completeOrigins(digest string) OriginBatch {
	origin := &Origin{VCS: "git", URL: "https://github.com/o/r", Hash: strings.Repeat("a", 40)}
	return OriginBatch{InventorySHA: digest, Limit: 2, Results: []Resolution{
		{Path: "first", Version: "v1.0.0", Origin: origin}, {Path: "second", Version: "v2.0.0", Origin: origin}}}
}

func TestEnrichmentCreatesASeparateCompleteGraphAndPreservesActiveInputs(t *testing.T) {
	_, digest := provenanceFixture(t)
	if err := records.Save("origins-0000.json", completeOrigins(digest)); err != nil {
		t.Fatal(err)
	}
	if err := records.Save("plan.json", "unchanged active plan"); err != nil {
		t.Fatal(err)
	}
	report, err := Enrich()
	if err != nil || len(missingOrigins(report)) != 0 {
		t.Fatalf("incomplete enriched report: %v", err)
	}
	if got, err := records.Read("inventory.json", new(Report)); err != nil || got != digest {
		t.Fatalf("original inventory changed: %s %v", got, err)
	}
	var plan string
	if _, err := records.Read("plan.json", &plan); err != nil || plan != "unchanged active plan" {
		t.Fatalf("active plan changed: %q %v", plan, err)
	}
	var saved Report
	if _, err := records.Read("resolved-inventory.json", &saved); err != nil || len(missingOrigins(saved)) != 0 {
		t.Fatalf("enriched inventory not saved: %v", err)
	}
}

func TestEnrichmentRejectsStaleMismatchedAndIncompleteResolutions(t *testing.T) {
	_, digest := provenanceFixture(t)
	stale, mismatched, incomplete := completeOrigins("stale"), completeOrigins(digest), completeOrigins(digest)
	mismatched.Results[0].Version = "v2.0.0"
	incomplete.Results = incomplete.Results[:1]
	for _, batch := range []OriginBatch{stale, mismatched, incomplete} {
		if err := records.Save("origins-0000.json", batch); err != nil {
			t.Fatal(err)
		}
		if _, err := Enrich(); err == nil {
			t.Fatalf("accepted incomplete or foreign checkpoint %+v", batch)
		}
	}
}

func TestOriginMergeNeverOverwritesAConflictingCommit(t *testing.T) {
	report, digest := provenanceFixture(t)
	resolved := map[string]*Origin{"first@v1.0.0": {VCS: "git", URL: "https://github.com/o/r", Hash: strings.Repeat("b", 40)}}
	if err := mergeBatch(resolved, completeOrigins(digest), digest, missingOrigins(report)); err == nil {
		t.Fatal("different commit silently replaced previous provenance")
	}
}
