package engines_test

import (
	"fmt"
	"reflect"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/runtime/engines"
)

func BenchmarkCompileFleet(b *testing.B) {
	for _, mixed := range []bool{false, true} {
		for _, count := range []int{1, 10, 100, 1000} {
			b.Run(fmt.Sprintf("mixed=%t/nodes=%d", mixed, count), func(b *testing.B) {
				top, state := fleetFixture(count, mixed)
				b.ReportAllocs()
				for b.Loop() {
					compiler := engines.NewCompiler(top)
					for _, id := range top.Order {
						data, err := compiler.Node(id, state)
						if err != nil || data.Generation == "" {
							b.Fatal("fleet compilation failed", err)
						}
					}
				}
				b.ReportMetric(float64(count), "nodes/op")
			})
		}
	}
}

func TestNodeGenerationCompatibility(t *testing.T) {
	// Canonical generations include the per-packet UDP exit policy. Relays keep
	// their pre-indexing generations; exits deliberately change for this policy.
	want := map[bool][]string{
		false: {"89ff936cdd7b06fc8cff7cb211a0e0374343068f2815dd9f82c04744c1f5fbf9", "24acc6b45e0eac947f87c5009da4d61a20b3f25d35374dae3d6c81f47ab5e377", "db231cab379622d9fdd8b6c4c87b6f57901411930853fe2c6b1b9d3eb5365417"},
		true:  {"178dc4256bfb7a86b77d21456b9c94898edbe491c377af46433818d302514ec5", "a6ced1357d387d9e77c6178ddfaa65276aec0a7645f2f28468f599fd5a66f76a", "2e24eeee62a2ed6158a10a6f50a2f2ee73dfbf4f45c2bbbeba44094fe7b5a567"},
	}
	for _, mixed := range []bool{false, true} {
		top, state := fleetFixture(3, mixed)
		compiler := engines.NewCompiler(top)
		for index, id := range top.Order {
			data, err := compiler.Node(id, state)
			if err != nil {
				t.Fatal(err)
			}
			if data.Generation != want[mixed][index] {
				t.Errorf("mixed=%t node=%s changed generation: %s", mixed, id, data.Generation)
			}
		}
	}
}

func TestFreshCompilerPreservesTopologyWideSchema(t *testing.T) {
	top, state := fleetFixture(2, false)
	top.Successors = map[string]string{}
	before, err := engines.NewCompiler(top).Node("node-0000", state)
	if err != nil || before.SchemaVersion != 1 {
		t.Fatal("Xray-only fleet lost its legacy schema", err)
	}
	name := "node-0001.second"
	endpoint := top.Services[name]
	endpoint.Service.Protocol = "snell-v5"
	top.Services[name] = endpoint
	top.Nodes[endpoint.Node].Services[endpoint.ID].Protocol = endpoint.Service.Protocol
	secret := state.Services[name]
	secret.Protocol = endpoint.Service.Protocol
	state.Services[name] = secret
	// A remote sing-box node changes the schema even for this Xray-only exit.
	after, err := engines.NewCompiler(top).Node("node-0000", state)
	if err != nil || after.SchemaVersion != 2 || len(after.Programs) != 1 || after.Programs[0].Name != "xray" {
		t.Fatal("fresh fleet index lost the remote engine or cached an old schema", err)
	}
	if after.Generation == before.Generation {
		t.Fatal("schema change retained the old generation")
	}
}

func TestCompilerDoesNotRetainSecretsOrReturnedPrograms(t *testing.T) {
	top, state := fleetFixture(2, true)
	compiler := engines.NewCompiler(top)
	first, err := compiler.Node("node-0000", state)
	if err != nil {
		t.Fatal(err)
	}
	first.Programs[0].Config["log"] = model.Object{"modified": true}
	again, err := compiler.Node("node-0000", state)
	if err != nil || again.Generation != first.Generation || reflect.DeepEqual(first.Programs[0].Config, again.Programs[0].Config) {
		t.Fatal("a caller's result mutation changed the next compilation", err)
	}
	secret := state.Services["node-0000.first"]
	secret.Password = fixturePassword("replacement")
	state.Services["node-0000.first"] = secret
	rotated, err := compiler.Node("node-0000", state)
	if err != nil || rotated.Generation == again.Generation {
		t.Fatal("compiler retained stale credentials", err)
	}
}
