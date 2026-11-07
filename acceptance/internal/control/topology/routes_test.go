package topology_test

import (
	"fmt"
	"slices"
	"testing"

	"vpn-script/internal/control/topology"
	"vpn-script/internal/core/model"
)

func TestEightNodes(t *testing.T) {
	top, err := topology.Load("../../../examples/nhop/topology.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for index := 4; index <= 8; index++ {
		name := fmt.Sprintf("hop%d", index)
		node := *top.Nodes["relay"]
		node.Address = fmt.Sprintf("203.0.113.%d", index)
		top.Nodes[name] = &node
		for id, suffix := range map[string]string{"main": ".reality", "alternative": ".shadowsocks"} {
			path := top.Routes[id].Path
			top.Routes[id].Path = slices.Insert(path, len(path)-1, name+suffix)
		}
	}
	expanded, err := topology.Build(top.Definition)
	if err != nil || len(expanded.Order) != 8 || expanded.Order[0] != "exit" || expanded.Order[7] != "entry" {
		t.Fatalf("N-hop expansion failed: %v", err)
	}
}

func TestAmbiguousSuccessorRejected(t *testing.T) {
	top, err := topology.Load("../../../examples/nhop/topology.yaml")
	if err != nil {
		t.Fatal(err)
	}
	top.Routes["ambiguous"] = &model.Route{Path: []string{"entry.reality", "exit.reality"}}
	if _, err := topology.Build(top.Definition); err == nil {
		t.Fatal("one listener accepted two different successors")
	}
}

func TestExplicitSSH22HasNoImplicitClosed22(t *testing.T) {
	top, err := topology.Load("../../../examples/nhop/topology.yaml")
	if err != nil {
		t.Fatal(err)
	}
	top.Nodes["entry"].SSH.Port = 22
	top.Nodes["entry"].SSH.ClosedPorts = nil
	top, err = topology.Build(top.Definition)
	if err != nil || len(top.Nodes["entry"].SSH.ClosedPorts) != 0 {
		t.Fatalf("SSH port 22 incorrectly marked closed: %v", err)
	}
}

func TestExpectedExitAddressNormalizationAndScope(t *testing.T) {
	for _, test := range []struct{ input, expected string }{
		{"::ffff:198.51.100.42", "198.51.100.42"},
		{"2001:0db8::42", "2001:db8::42"},
		{"fe80::1%eth0", ""},
	} {
		top, err := topology.Load("../../../examples/nhop/topology.yaml")
		if err != nil {
			t.Fatal(err)
		}
		top.Routes["main"].ExpectedExitIP = test.input
		normalized, err := topology.Build(top.Definition)
		if test.expected == "" {
			if err == nil {
				t.Error("scoped expected exit accepted")
			}
		} else if err != nil || normalized.Routes["main"].ExpectedExitIP != test.expected {
			t.Errorf("expected exit was not normalized: %v", err)
		}
	}
}
