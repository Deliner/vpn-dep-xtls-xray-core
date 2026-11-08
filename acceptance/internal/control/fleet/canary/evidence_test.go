package canary

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"vpn-script/internal/control/integration/traffic"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
)

func (f *fixture) traffic(t *testing.T, phase string) {
	t.Helper()
	input := probeInput{Phase: phase}
	for _, name := range []string{"shadowsocks", "tuic"} {
		config, profile := engines.Client(f.topology, name, f.secrets)
		if profile.HopCount != 3 || profile.EntryNode != "fleet-a" || profile.ExitNode != "fleet-c" {
			t.Fatal("traffic must traverse the explicit three-machine route")
		}
		input.Clients = append(input.Clients, probeClient{ID: name, Engine: profile.Engine, Port: profile.SOCKSPort, Config: config})
	}
	name := phase + "-probe.json"
	f.putJSON(t, "fleet-c", name, input)
	f.guest(t, "fleet-c", "cp", "--", f.remote+"/"+name, f.remote+"/probe.json")
	ok := t.Run("traffic-"+phase, func(t *testing.T) {
		raw := f.guestTest(t, "fleet-c", "TestFleetGuestTraffic")
		require(t, safeio.NewJSON(filepath.Join(f.root, phase+"-probe-output.json"), string(raw)))
	})
	report := f.saveRemote(t, "fleet-c", f.remote+"/"+phase+"-traffic.json", phase+"-traffic.json")
	f.report.Traffic[phase] = decode[traffic.Report](t, report)
	for _, name := range []string{"shadowsocks", "tuic"} {
		f.saveRemote(t, "fleet-c", f.remote+"/"+phase+"-"+name+".json.log", phase+"-"+name+".log")
	}
	if !ok {
		f.saveRemote(t, "fleet-c", f.remote+"/"+phase+"-recovery.json", phase+"-recovery.json")
		t.Fatal("three-hop traffic failed; first failure and bounded recovery observations retained")
	}
}

func (f *fixture) installed(t *testing.T, phase string) {
	t.Helper()
	f.report.Revisions[phase] = f.inventory(t)
	hash, err := safeio.FileHash(filepath.Join(f.root, "nhop-fleet"))
	require(t, err)
	f.report.AgentSHA256 = hash
	for _, node := range f.report.Revisions[phase] {
		name := phase + "-expected.json"
		f.putJSON(t, node.NodeID, name, f.current[node.NodeID])
		f.guest(t, node.NodeID, "cp", "--", f.remote+"/"+name, f.remote+"/expected.json")
		f.guestTest(t, node.NodeID, "TestFleetGuestInventory")
		raw := f.saveRemote(t, node.NodeID, nodeBase+"/state.json", phase+"-installed.json")
		state := decode[installer.State](t, raw)
		if node.Status == nil || node.Status.Phase != "ready" || state.Generation != f.current[node.NodeID].Generation || node.Status.Generation != state.Generation || state.Files["nhop-node"] != hash {
			t.Fatal("installed supervisor, generation and real agent report disagree", node.NodeID)
		}
		if f.property(t, node.NodeID, "vpn-nhop.service", "SubState") != "running" {
			t.Fatal("real VPN unit is not running", node.NodeID)
		}
	}
}

func (f *fixture) gated(t *testing.T, phase string, before []wire.Node, pids map[string]string) {
	t.Helper()
	after := f.inventory(t)
	f.report.Revisions[phase] = after
	for _, current := range after {
		if current.NodeID == "fleet-c" {
			continue
		}
		var prior wire.Node
		for _, node := range before {
			if node.NodeID == current.NodeID {
				prior = node
			}
		}
		if current.Status == nil || prior.Status == nil || !reflect.DeepEqual(prior.Desired, current.Desired) || current.Status.Generation != prior.Status.Generation || f.property(t, current.NodeID, "vpn-nhop.service", "MainPID") != pids[current.NodeID] {
			t.Fatal("a later-stage node changed after the canary gate closed", current.NodeID)
		}
	}
	if len(after) != 3 {
		t.Fatal(fmt.Sprintf("expected three agents, got %d", len(after)))
	}
}

func (f *fixture) pids(t *testing.T) map[string]string {
	t.Helper()
	result := map[string]string{}
	for _, node := range nodes {
		result[node] = f.property(t, node, "vpn-nhop.service", "MainPID")
	}
	return result
}
