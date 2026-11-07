package mirroring

import (
	"strings"
	"testing"
)

func TestGitSourceRequiresReviewedUnambiguousIdentity(t *testing.T) {
	row := []string{"vpn-dep-fixture", "https://example.org/source", strings.Repeat("a", 40)}
	if _, err := selectSource([][]string{row}, row[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := selectSource([][]string{row}, "unreviewed"); err == nil {
		t.Fatal("accepted unknown source")
	}
	if _, err := selectSource([][]string{row, row}, row[0]); err == nil {
		t.Fatal("accepted duplicate source")
	}
	for index, replacement := range []string{"../escape", "https://user@example.org/source", "short"} {
		broken := append([]string{}, row...)
		broken[index] = replacement
		if _, err := selectSource([][]string{broken}, broken[0]); err == nil {
			t.Fatal("accepted ambiguous source")
		}
	}
}
