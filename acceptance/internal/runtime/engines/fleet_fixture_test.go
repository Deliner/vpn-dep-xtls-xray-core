package engines_test

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"slices"

	"vpn-script/internal/core/model"
)

// Deterministic synthetic credentials keep generation compatibility measurable.
// Each listener has its own key; these fixtures never contact the documentation IPs.
func fleetFixture(nodes int, mixed bool) (*model.Topology, model.Secrets) {
	top := &model.Topology{Definition: model.Definition{Version: 1, Name: "fleet", Nodes: map[string]*model.Node{}},
		Services: map[string]model.Endpoint{}, Successors: map[string]string{}}
	state := model.Secrets{SchemaVersion: 1, TopologyID: top.Name, Services: map[string]model.Secret{}}
	for index := range nodes {
		id := fmt.Sprintf("node-%04d", index)
		node := &model.Node{Address: "192.0.2.1", Firewall: "preserve", SSH: model.SSH{Port: 22222, ClosedPorts: []int{22}}, Services: map[string]*model.Service{}}
		top.Nodes[id] = node
		top.Order = append(top.Order, id)
		for offset, serviceID := range []string{"first", "second"} {
			kind := model.Shadowsocks
			if offset == 1 && mixed {
				kind = "snell-v5"
			}
			service := model.Service{Protocol: kind, Listen: "127.0.0.1", Port: 40000 + offset}
			name := id + "." + serviceID
			node.Services[serviceID] = &service
			top.Services[name] = model.Endpoint{Node: id, ID: serviceID, Service: service}
			state.Services[name] = model.Secret{Protocol: kind, Password: fixturePassword(name), BridgePassword: fixturePassword("bridge-" + name)}
			if index+1 < nodes {
				top.Successors[name] = fmt.Sprintf("node-%04d.%s", index+1, []string{"second", "first"}[offset])
			}
		}
	}
	slices.Reverse(top.Order)
	return top, state
}

func fixturePassword(name string) string {
	digest := sha256.Sum256([]byte("public-test-fixture/" + name))
	return base64.StdEncoding.EncodeToString(digest[:])
}
