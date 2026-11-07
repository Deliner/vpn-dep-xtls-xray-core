package selection

import (
	"strings"
	"testing"

	"vpn-script/internal/control/integration/evidence"
)

func options() Options {
	return Options{Servers: map[string]evidence.Input{
		"xray": {Path: "/default/xray", ExpectedVersion: "1"}, "sing-box": {Path: "/default/sing-box", ExpectedVersion: "2"}},
		Clients: map[string]evidence.Input{"xray": {Path: "/client/xray", ExpectedVersion: "1"}},
		Updated: map[string]evidence.Input{
			"xray": {Path: "/updated/xray", ExpectedVersion: "1"}, "sing-box": {Path: "/updated/sing-box", ExpectedVersion: "2"}},
		UpdatedNodes: []string{"node2"}}
}

func TestClientAndPerNodeSelectionsRemainIndependent(t *testing.T) {
	set, err := New([]string{"node0", "node1", "node2"}, options())
	if err != nil {
		t.Fatal(err)
	}
	if set.Inputs[set.Clients["xray"]].Path != "/client/xray" || set.Inputs[set.Clients["sing-box"]].Path != "/default/sing-box" {
		t.Fatal("client overrides or inheritance were lost")
	}
	for _, node := range []string{"node0", "node1", "node2"} {
		prefix := "/default/"
		if node == "node2" {
			prefix = "/updated/"
		}
		for _, engine := range []string{"xray", "sing-box"} {
			if set.Inputs[set.Nodes[node][engine]].Path != prefix+engine {
				t.Fatalf("wrong %s runtime for %s", engine, node)
			}
		}
	}
	if len(set.Inputs) != 6 {
		t.Fatalf("missing role identities: %+v", set.Inputs)
	}
	set.Nodes["node0"]["xray"] = "changed"
	if set.Nodes["node1"]["xray"] != "server/xray" {
		t.Fatal("node assignments share a mutable map")
	}
}

func TestOnlyParticipatingRolesRequireCapturedIdentities(t *testing.T) {
	input := options()
	input.UpdatedNodes = []string{"node0", "node1"}
	set, err := New([]string{"node0", "node1"}, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Inputs) != 4 {
		t.Fatalf("unused server binaries included as executed evidence: %+v", set.Inputs)
	}
	if _, exists := set.Inputs["server/xray"]; exists {
		t.Fatal("unused server identity captured")
	}
	input.Clients["xray"] = evidence.Input{Path: "changed"}
	if set.Inputs["client/xray"].Path != "/client/xray" {
		t.Fatal("selection aliases mutable input maps")
	}
}

func TestInvalidSelectionsCannotSilentlyFallBack(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Options)
		want   string
	}{
		{"unknown node", func(o *Options) { o.UpdatedNodes = []string{"node9"} }, "unknown"},
		{"duplicate update", func(o *Options) { o.UpdatedNodes = []string{"node2", "node2"} }, "duplicate"},
		{"missing engine", func(o *Options) { delete(o.Updated, "sing-box") }, "every engine"},
		{"empty updated path", func(o *Options) { o.Updated["xray"] = evidence.Input{} }, "missing path"},
		{"empty client path", func(o *Options) { o.Clients["xray"] = evidence.Input{} }, "missing path"},
		{"unknown engine", func(o *Options) { o.Clients["other"] = evidence.Input{Path: "other"} }, "unknown"},
		{"unused updates", func(o *Options) { o.UpdatedNodes = nil }, "without selected nodes"},
		{"no servers", func(o *Options) { o.Servers = nil }, "no server"},
		{"different expectation", func(o *Options) { o.Clients["xray"] = evidence.Input{Path: "client", ExpectedVersion: "99"} }, "mismatched"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := options()
			test.change(&input)
			if _, err := New([]string{"node0", "node1", "node2"}, input); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("invalid selection accepted or misdiagnosed: %v", err)
			}
		})
	}
}

func TestNodeIdentitiesMustBeUnambiguous(t *testing.T) {
	for _, nodes := range [][]string{nil, {""}, {"node2", "node2"}} {
		if _, err := New(nodes, options()); err == nil {
			t.Fatalf("ambiguous nodes accepted: %v", nodes)
		}
	}
}
