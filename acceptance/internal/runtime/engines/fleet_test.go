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
	// Runtime archive identity is part of the canonical generation, including on
	// relays. These snapshots include the published maintained Xray Linux assets.
	want := map[bool][]string{
		false: {"d1d7b9377caa7a928f0780c12950b200df3adad4afeccc073038f52d7832ad83", "366baf966884bc59ae50a8c1d5c23254d7fe3504ca42783506c428e7995fa888", "3355ffeaa5a1210643d028a3e2c841569c1b818868b9c5cac05c980384f9a399"},
		true:  {"13e9fe3a75a05131caded51fc559dd5d96e0ba8fac7496a27b5bab3a02d6db9e", "0d281d9b09758f7a2f0d18308fa37f15addb5087932c0fb2cc3c4a716afd5d7a", "0b47d8a97dcb4bcc86ae1c289a4b91eeae44691fff0a5d762e9cbc8223446a40"},
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
