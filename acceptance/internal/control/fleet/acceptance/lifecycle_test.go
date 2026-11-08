package acceptance

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"vpn-script/internal/control/integration/traffic"
	"vpn-script/internal/node/fleet/service"
)

func TestNativeFleetLifecycle(t *testing.T) {
	root, binary := guard(t)
	f := &fixture{root: root, binary: binary, report: report{Version: 1, Platform: runtime.GOOS + "/" + runtime.GOARCH, Traffic: map[string]traffic.Report{}}}
	t.Cleanup(func() { f.saveReport(t) })
	f.controller = startController(t, root)
	t.Cleanup(func() { f.cleanup(t) })
	f.topologyFixture(t)
	for _, phase := range []struct {
		name string
		run  func(*testing.T)
	}{{"initial", f.initial}, {"update", f.update}, {"activation-rollback", f.rollback},
		{"controller-outage", f.outage}, {"agent-crash", f.crash}, {"revocation", f.revoke}} {
		if !t.Run(phase.name, phase.run) {
			return
		}
	}
	f.cli(t, "agent-service-stop")
	f.cli(t, "agent-service-remove")
	if _, err := os.Lstat(service.UnitPath); !os.IsNotExist(err) {
		t.Fatal("agent service removal left its unit registered")
	}
	for _, path := range []string{filepath.Join(agentState, "connection.json"), filepath.Join(agentState, "agent/state.json"), filepath.Join(service.Directory, "releases")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("agent removal deleted durable state or versions")
		}
	}
	f.report.RemovedStateRetained = true
}

func (f *fixture) initial(t *testing.T) {
	f.enroll(t)
	data := f.configuration(t, false)
	revision := f.publish(t, data)
	node := f.waitStatus(t, "ready", revision)
	if node.Status.Generation != data.Generation || node.Status.AppliedRevision != revision {
		t.Fatal("initial reported generation differs from the signed assignment")
	}
	f.verifyInstallation(t, data)
	f.report.Generations = append(f.report.Generations, data.Generation)
	f.traffic(t, "initial")
}

func (f *fixture) update(t *testing.T) {
	f.updated = f.configuration(t, true)
	revision := f.publish(t, f.updated)
	f.waitStatus(t, "ready", revision)
	f.verifyInstallation(t, f.updated)
	f.report.Generations = append(f.report.Generations, f.updated.Generation)
	if f.report.Generations[0] == f.updated.Generation {
		t.Fatal("changed listener ports did not create a new generation")
	}
	f.traffic(t, "updated")
}
