package records

import (
	"os"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestReadHashesTheDecodedBytes(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := Save("record.json", map[string]int{"count": 7}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("build/dependencies/record.json")
	if err != nil {
		t.Fatal(err)
	}
	var record struct{ Count int }
	digest, err := Read("record.json", &record)
	if err != nil || record.Count != 7 || digest != safeio.Hash(raw) {
		t.Fatalf("record=%+v digest=%s error=%v", record, digest, err)
	}
}

func TestReadRejectsUnknownFieldsAndTrailingContent(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := Save("record.json", nil); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"Count":1,"unexpected":2}`, `{"Count":1}{}`, `{"Count":1} trailing`} {
		if err := os.WriteFile("build/dependencies/record.json", []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		var record struct{ Count int }
		if _, err := Read("record.json", &record); err == nil {
			t.Fatalf("accepted invalid record %s", raw)
		}
	}
}

func TestRecordsRejectPathsOutsideTheirDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, name := range []string{"", ".", "..", "../other.json", "nested/record.json", "/record.json"} {
		if err := Save(name, nil); err == nil {
			t.Fatalf("save accepted %q", name)
		}
		if _, err := Read(name, new(any)); err == nil {
			t.Fatalf("read accepted %q", name)
		}
	}
}
