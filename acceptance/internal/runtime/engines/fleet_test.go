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
	// relays. These snapshots include the published maintained Xray masque-sack-1 assets.
	want := map[bool][]string{
		false: {"c7b3a8a389b19078192f5dd58cd34338e8bb847d5b5486f817b98720ed9907ee", "c2f814ca4af1ab740b68372e0b1ff074b2a89e435f7d50874e8bccec83e62737", "a9e72191bef78f5c2932d64c7895c13e7e61f3ce2b3852f011258c887065cb69"},
		true:  {"53a3622c6aca9133c92ed11a584381b1c2b1666e2cdec1b8678c04135ad4aaca", "4c79b46962d0510c719af15ccb26a14ba3408fe4c476b32e34f477e33d387378", "59978c0e0468c6009e89fec2375907511195dd2f2d7378d3c4eabb7cf8314d93"},
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
