package scope

import "testing"

func TestMaintenanceScopeRejectsUnownedAndAmbiguousTargets(t *testing.T) {
	valid := []string{"owner/source", "Deliner/vpn-dep-source", "10", "20", "base", "fix"}
	if _, err := parse([][]string{valid}); err != nil {
		t.Fatal(err)
	}
	for _, rows := range [][][]string{nil, {valid, valid}, {{"short"}}, {{"o/r", "someone/source", "10", "20", "base", "fix"}}, {{"o/r", "Deliner/vpn-dep-source", "10", "0", "base", "fix"}}} {
		if _, err := parse(rows); err == nil {
			t.Fatalf("accepted unsafe maintenance scope: %v", rows)
		}
	}
}
