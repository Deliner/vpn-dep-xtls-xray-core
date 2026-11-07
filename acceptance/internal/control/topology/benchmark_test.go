package topology_test

import (
	"testing"

	"vpn-script/internal/control/topology"
)

func BenchmarkLoadProtocolTopology(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		if _, err := topology.Load("../../../examples/nhop/protocols.yaml"); err != nil {
			b.Fatal(err)
		}
	}
}
