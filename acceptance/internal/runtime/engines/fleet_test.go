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
	// relays. Snapshots include maintained Xray masque-sack-1 and sing-box Cronet/zlib/TUIC assets.
	want := map[bool][]string{
		false: {"c7b3a8a389b19078192f5dd58cd34338e8bb847d5b5486f817b98720ed9907ee", "c2f814ca4af1ab740b68372e0b1ff074b2a89e435f7d50874e8bccec83e62737", "a9e72191bef78f5c2932d64c7895c13e7e61f3ce2b3852f011258c887065cb69"},
		true:  {"e03385de2dd8cdd7c94bfa40a1983ad4f3f32643ca06903d33a3af9821287b6e", "a0fd00117e783a1315617a9d4b22d99d2de89b0afc678945144d0ebca0a7c132", "05966c1abc798d25887015b817659fe427da5771a056759edff93f962b1902b2"},
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
