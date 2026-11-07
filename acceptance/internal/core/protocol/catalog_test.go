package protocol_test

import (
	"testing"

	"vpn-script/internal/core/protocol"
)

func TestCatalogIsOrderedUniqueAndSearchable(t *testing.T) {
	all := protocol.All()
	if len(all) == 0 {
		t.Fatal("empty protocol catalog")
	}
	for index, expected := range all {
		if expected.ID == "" || index > 0 && all[index-1].ID >= expected.ID {
			t.Fatalf("catalog IDs are empty, duplicate or unordered at %d", index)
		}
		actual, found := protocol.Lookup(expected.ID)
		if !found || actual != expected || protocol.Engine(expected.ID) != expected.Engine {
			t.Fatalf("lookup lost catalog metadata for %s", expected.ID)
		}
	}
	for _, id := range []string{"", "VLESS-REALITY", "vless-", "vless-reality-unknown", "zz-unknown"} {
		if actual, found := protocol.Lookup(id); found || actual != (protocol.Spec{}) || protocol.Engine(id) != "" {
			t.Fatalf("unknown or partial ID %q matched a protocol", id)
		}
	}
}

func TestCatalogCallersOwnTheirCopies(t *testing.T) {
	expected := protocol.All()[0]
	for range 8 {
		t.Run("independent-caller", func(t *testing.T) {
			t.Parallel()
			all := protocol.All()
			all[0].ID, all[0].Engine, all[0].Status = "changed", "changed", "changed"
			actual, found := protocol.Lookup(expected.ID)
			if !found || actual != expected || protocol.All()[0] != expected {
				t.Fatal("one caller changed another caller's catalog")
			}
			actual.Engine = "changed"
			if protocol.Engine(expected.ID) != expected.Engine {
				t.Fatal("returned lookup value aliases the catalog")
			}
		})
	}
}

func BenchmarkCatalogLookup(b *testing.B) {
	for _, id := range []string{"vless-reality", "unknown-protocol"} {
		b.Run(id, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				protocol.Lookup(id)
			}
		})
	}
}

func BenchmarkCatalogList(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		protocol.All()
	}
}
