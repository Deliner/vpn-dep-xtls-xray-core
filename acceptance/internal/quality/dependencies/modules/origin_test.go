package modules

import "testing"

func TestOriginLookupUsesReplacementAndKeepsVersionsSeparate(t *testing.T) {
	modules := []Module{{Main: true, Path: "main"}, {Path: "upstream", Version: "v1", Replace: &Module{Path: "fork", Version: "v2"}},
		{Path: "other", Version: "v1", Replace: &Module{Path: "fork", Version: "v2"}}, {Path: "fork", Version: "v3"}}
	requests, targets := originRequests(modules)
	if len(requests) != 2 || requests[0] != "fork@v2" || requests[1] != "fork@v3" {
		t.Fatal(requests)
	}
	raw := `{"Path":"fork","Version":"v2","Origin":{"URL":"https://github.com/o/r","Hash":"first"}}
{"Path":"fork","Version":"v3","Origin":{"URL":"https://github.com/o/r","Hash":"second"}}`
	if err := applyOrigins([]byte(raw), targets); err != nil {
		t.Fatal(err)
	}
	if modules[1].Origin != nil || modules[1].Replace.Origin.Hash != "first" || modules[2].Replace.Origin.Hash != "first" || modules[3].Origin.Hash != "second" {
		t.Fatalf("origin assigned to the wrong module version: %+v", modules)
	}
}

func TestOriginLookupRejectsMissingExtraAndDuplicateResults(t *testing.T) {
	valid := `{"Path":"dep","Version":"v1"}`
	for _, raw := range []string{"", valid + valid, valid + `{}`, `{"Path":"dep","Version":"v2"}`, `{"Path":"dep","Version":"v1","Error":{"Err":"failed"}}`} {
		_, targets := originRequests([]Module{{Path: "dep", Version: "v1"}})
		if err := applyOrigins([]byte(raw), targets); err == nil {
			t.Fatalf("accepted incomplete or mismatched response %q", raw)
		}
	}
}
