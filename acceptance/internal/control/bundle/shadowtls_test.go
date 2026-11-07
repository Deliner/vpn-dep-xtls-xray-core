package bundle_test

import (
	"fmt"
	"strings"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/engines"
)

func TestShadowTLSOwnsExplicitPrivateTCPListeners(t *testing.T) {
	top, state := shadowFixture(t)
	data := shadowNode(t, top, state)
	must(t, manifest.Validate(data))
	used := map[int]bool{40000: true, 40001: true, 40002: true, 40003: true}
	count := 0
	for _, listener := range data.Listeners {
		if !listener.Internal || (listener.Tag != "node.one" && listener.Tag != "node.two") {
			continue
		}
		count++
		if listener.Address != "127.0.0.1" || listener.Network != "tcp" || used[listener.Port] || listener.Port < 40000 {
			t.Fatalf("unsafe or conflicting private listener: %+v", listener)
		}
		used[listener.Port] = true
		inner := shadowInner(t, data, listener.Tag)
		if inner["network"] != "tcp" || inner["listen"] != listener.Address || fmt.Sprint(inner["listen_port"]) != fmt.Sprint(listener.Port) {
			t.Fatal("private listener differs from engine configuration")
		}
		if strings.Contains(installer.UFWProfile(data), fmt.Sprintf("%d/", listener.Port)) {
			t.Fatal("private listener included in public firewall profile")
		}
	}
	if count != 2 {
		t.Fatalf("expected two private ShadowTLS listeners, got %d", count)
	}
	if data.Generation != shadowNode(t, top, state).Generation {
		t.Fatal("rerender changed internal port assignment")
	}
}

func TestManifestRejectsUnboundedShadowTLSInner(t *testing.T) {
	top, state := shadowFixture(t)
	data := shadowNode(t, top, state)
	for _, tag := range []string{"node.one", "node.two"} {
		inner := shadowInner(t, data, tag)
		delete(inner, "listen")
		delete(inner, "listen_port")
	}
	var public []model.Listener
	for _, listener := range data.Listeners {
		if !listener.Internal {
			public = append(public, listener)
		}
	}
	data.Listeners = public
	resign(t, &data)
	if manifest.Validate(data) == nil {
		t.Fatal("accepted undeclared ephemeral ShadowTLS listeners")
	}
}

func TestShadowTLSInternalPortExhaustion(t *testing.T) {
	top, state := shadowFixture(t)
	node := top.Nodes["node"]
	node.BridgePortStart = 65000
	for port := 65000; port <= 65535; port++ {
		node.SSH.ClosedPorts = append(node.SSH.ClosedPorts, port)
	}
	if _, err := engines.Node(top, "node", state); err == nil {
		t.Fatal("accepted topology without a free internal listener port")
	}
}

func TestManifestEnforcesShadowTLSPrivateTransport(t *testing.T) {
	for _, change := range []string{"public-bind", "udp", "default-network"} {
		t.Run(change, func(t *testing.T) {
			top, state := shadowFixture(t)
			data := shadowNode(t, top, state)
			inner := shadowInner(t, data, "node.one")
			if change == "public-bind" {
				inner["listen"] = "0.0.0.0"
				for i, listener := range data.Listeners {
					if listener.Tag == "node.one" && listener.Internal {
						data.Listeners[i].Address = "0.0.0.0"
					}
				}
			} else if change == "udp" {
				inner["network"] = "udp"
			} else {
				delete(inner, "network")
			}
			resign(t, &data)
			if manifest.Validate(data) == nil {
				t.Fatal("accepted unsafe or inconsistent private listener")
			}
		})
	}
}

func shadowFixture(t *testing.T) (*model.Topology, model.Secrets) {
	t.Helper()
	node := &model.Node{Address: "192.0.2.1", Firewall: "ufw", BridgePortStart: 40000,
		SSH: model.SSH{Port: 40000, ClosedPorts: []int{40001}}, Services: map[string]*model.Service{}}
	top := &model.Topology{Definition: model.Definition{Name: "shadow", Nodes: map[string]*model.Node{"node": node}},
		Services: map[string]model.Endpoint{}, Successors: map[string]string{}}
	state := model.Secrets{Services: map[string]model.Secret{}}
	for index, id := range []string{"one", "two"} {
		service := &model.Service{Protocol: "shadowtls-v3", Listen: "0.0.0.0", Port: 40002 + index,
			ServerName: "donor.example.invalid", Target: "donor.example.invalid:443"}
		name := "node." + id
		node.Services[id] = service
		top.Services[name] = model.Endpoint{Node: "node", ID: id, Service: *service}
		secret, err := credentials.NewSecret(service.Protocol)
		must(t, err)
		state.Services[name] = secret
	}
	return top, state
}

func shadowNode(t *testing.T, top *model.Topology, state model.Secrets) model.NodeConfig {
	t.Helper()
	data, err := engines.Node(top, "node", state)
	must(t, err)
	for i := range data.Programs {
		data.Programs[i].Config = engines.Object(data.Programs[i].Config)
	}
	return data
}

func shadowInner(t *testing.T, data model.NodeConfig, tag string) model.Object {
	t.Helper()
	for _, program := range data.Programs {
		for _, raw := range program.Config["inbounds"].([]any) {
			inbound := raw.(map[string]any)
			if inbound["tag"] == tag && inbound["type"] == "shadowsocks" {
				return inbound
			}
		}
	}
	t.Fatalf("missing ShadowTLS inner %s", tag)
	return nil
}

func resign(t *testing.T, data *model.NodeConfig) {
	t.Helper()
	data.Generation = ""
	var err error
	data.Generation, err = safeio.Digest(data)
	must(t, err)
}
