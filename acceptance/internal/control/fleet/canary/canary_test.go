package canary

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"vpn-script/internal/core/fleet/wire"
)

func TestFleetTUICRecovery(t *testing.T) {
	f := hostFixture(t)
	count, err := strconv.Atoi(os.Getenv("NHOP_FLEET_RECOVERY_CYCLES"))
	require(t, err)
	if count < 1 || count > 100 {
		t.Fatal("TUIC recovery requires 1..100 exit update cycles")
	}
	t.Cleanup(func() { f.save(t) })
	f.machines(t)
	f.capture(t)
	f.authority(t)
	f.enroll(t)
	f.topologyFixture(t)
	f.current = f.configuration(t, "recovery-even")
	f.createRollout(t, "install", f.current, 2)
	f.waitRollout(t, "install", "succeeded")
	f.installed(t, "initial")
	f.traffic(t, "initial")
	for cycle := 1; cycle <= count; cycle++ {
		f.exitRecovery(t, cycle)
	}
}

func (f *fixture) exitRecovery(t *testing.T, cycle int) {
	t.Helper()
	phase, variant := fmt.Sprintf("exit-update-%02d", cycle), "recovery-even"
	if cycle%2 == 1 {
		variant = "recovery-odd"
	}
	before, pids := f.inventory(t), f.pids(t)
	data := f.configuration(t, variant)["fleet-c"]
	observation := exitUpdate{Phase: phase, StartedAtNS: time.Now().UnixNano(), BeforePID: pids["fleet-c"]}
	desired := f.publishNode(t, phase, data)
	f.waitReady(t, "fleet-c", desired.Revision)
	observation.Revision, observation.ReadyAtNS = desired.Revision, time.Now().UnixNano()
	observation.AfterPID = f.property(t, "fleet-c", "vpn-nhop.service", "MainPID")
	if observation.BeforePID == observation.AfterPID || observation.AfterPID == "0" {
		t.Fatal("exit update did not replace the real VPN service")
	}
	f.current["fleet-c"] = data
	f.gated(t, phase, before, pids)
	f.installed(t, phase)
	f.report.ExitUpdates = append(f.report.ExitUpdates, observation)
	t.Logf("%s: revision %d ready after %s; entry/relay unchanged", phase, desired.Revision, time.Duration(observation.ReadyAtNS-observation.StartedAtNS))
	f.traffic(t, phase)
	f.report.ExitUpdates[len(f.report.ExitUpdates)-1].TrafficEndNS = time.Now().UnixNano()
}

func TestFleetCanary(t *testing.T) {
	f := hostFixture(t)
	t.Cleanup(func() { f.save(t) })
	f.machines(t)
	f.authority(t)
	f.enroll(t)
	f.topologyFixture(t)
	f.current = f.configuration(t, "initial")
	f.createRollout(t, "install", f.current, 2)
	f.waitRollout(t, "install", "succeeded")
	f.installed(t, "initial")
	f.traffic(t, "initial")
	f.failedCanary(t)
	f.restartRollout(t)
	f.cancelRollout(t)
}

func (f *fixture) failedCanary(t *testing.T) {
	t.Helper()
	before, pids := f.inventory(t), f.pids(t)
	f.createRollout(t, "failed-canary", f.configuration(t, "broken"), 2)
	value := f.waitRollout(t, "failed-canary", "failed")
	f.guest(t, "fleet-c", f.remote+"/nhop-fleet", "agent-service-stop")
	if value.PublishedNodes != 1 || value.FailedNode != "fleet-c" || value.Failure != "install_failed" {
		t.Fatal("failed activation did not stop the first canary stage", value)
	}
	f.gated(t, "failed-canary", before, pids)
	f.traffic(t, "canary-rolled-back")
	f.recoverCanary(t)
	f.installed(t, "recovered")
	f.report.FailedCanaryGated = true
}

func (f *fixture) restartRollout(t *testing.T) {
	t.Helper()
	before := f.inventory(t)
	updated := f.configuration(t, "update")
	f.createRollout(t, "restart", updated, 8)
	first := f.firstPublication(t, "restart")
	f.waitReady(t, "fleet-c", first.Nodes[0].PublishedRevision)
	if current := f.rollout(t, "restart"); current.PublishedNodes != 1 || current.Stage != 0 {
		t.Fatal("controller outage must start during the first canary observation window")
	}
	f.guest(t, "fleet-a", "systemctl", "stop", controllerUnit)
	f.traffic(t, "controller-offline")
	f.startController(t)
	f.waitRollout(t, "restart", "succeeded")
	after := f.inventory(t)
	for _, previous := range before {
		for _, current := range after {
			if current.NodeID == previous.NodeID && current.Desired.Revision != previous.Desired.Revision+1 {
				t.Fatal("controller restart duplicated a publication", current.NodeID)
			}
		}
	}
	if first.Nodes[0].PublishedRevision != f.report.Rollouts["restart"].Nodes[0].PublishedRevision {
		t.Fatal("durable canary receipt changed on restart")
	}
	f.current = updated
	f.installed(t, "updated")
	f.traffic(t, "updated")
	f.report.RestartRecovered = true
}

func (f *fixture) cancelRollout(t *testing.T) {
	t.Helper()
	before, pids := f.inventory(t), f.pids(t)
	updated := f.configuration(t, "cancel")
	f.createRollout(t, "canceled", updated, 30)
	first := f.firstPublication(t, "canceled")
	value := decode[wire.Rollout](t, f.admin(t, "rollout-cancel", "--id", "canceled"))
	if value.Phase != "canceled" || value.PublishedNodes != 1 {
		t.Fatal("cancel must retain only the already published canary", value)
	}
	f.waitReady(t, "fleet-c", first.Nodes[0].PublishedRevision)
	time.Sleep(2 * time.Second) // Cross another coordinator tick after cancellation.
	f.gated(t, "canceled", before, pids)
	f.current["fleet-c"] = updated["fleet-c"]
	f.installed(t, "canceled")
	f.report.Rollouts["canceled"] = f.rollout(t, "canceled")
	f.report.CancelGated = true
	f.traffic(t, "canceled")
}
