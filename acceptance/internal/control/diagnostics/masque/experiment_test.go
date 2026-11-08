package masque

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

func TestMASQUENetworkFaults(t *testing.T) {
	if os.Getenv("NHOP_MASQUE_NETEM_TEST") != "private-netns" {
		t.Skip("requires owned Linux namespace fault experiment")
	}
	f := setup(t)
	cycles := boundedEnvironment(t, "NHOP_MASQUE_CYCLES", 3, 20)
	workers := boundedEnvironment(t, "NHOP_MASQUE_WORKERS", 8, 64)
	packets := boundedEnvironment(t, "NHOP_MASQUE_PACKETS", 32, 256)
	for cycle := range cycles {
		f.cycle(t, cycle, workers, packets)
	}
}

func (f *fixture) cycle(t *testing.T, cycle, workers, packets int) {
	t.Helper()
	f.recover(t, cycle, "initial")
	states := []impairment{
		{Name: "baseline", MTU: 1500, Netem: []string{"delay", "0ms", "loss", "0%"}, Policy: "complete"},
		{Name: "delay", MTU: 1500, Netem: []string{"delay", "20ms", "5ms", "loss", "0%"}, Policy: "complete"},
		{Name: "loss", MTU: 1500, Netem: []string{"delay", "10ms", "loss", "2%"}, Policy: "observe"},
		{Name: "mtu-1400", MTU: 1400, Netem: []string{"delay", "5ms", "loss", "0%"}, Policy: "complete"},
		{Name: "mtu-1280", MTU: 1280, Netem: []string{"delay", "5ms", "loss", "0%"}, Policy: "complete"},
		{Name: "blackout", MTU: 1500, Netem: []string{"loss", "100%"}, Policy: "blocked"},
	}
	for _, state := range states {
		shape(t, state)
		f.phase(t, cycle, state, workers, packets)
		f.recover(t, cycle, "after-"+state.Name)
	}
	for _, role := range []string{"client", "server"} {
		f.restart(t, role)
		f.recover(t, cycle, "restart-"+role)
		f.phase(t, cycle, impairment{Name: "stable-after-" + role, MTU: 1500, Policy: "complete"}, workers, packets)
	}
}

func (f *fixture) phase(t *testing.T, cycle int, state impairment, workers, packets int) []flowResult {
	t.Helper()
	f.sequence++
	report := phaseReport{Nonce: f.sequence, Phase: state, Cycle: cycle, Started: time.Now().UTC()}
	report.Before = observeNetwork(t, state.MTU)
	report.Flows = measure(workers, packets, f.sequence, 2*time.Second)
	report.Finished = time.Now().UTC()
	report.DurationMS = report.Finished.Sub(report.Started).Milliseconds()
	report.After = observeNetwork(t, state.MTU)
	sort.Slice(report.Flows, func(i, j int) bool {
		if report.Flows[i].Network != report.Flows[j].Network {
			return report.Flows[i].Network < report.Flows[j].Network
		}
		return report.Flows[i].Worker < report.Flows[j].Worker
	})
	writeJSON(t, filepath.Join(f.directory, fmt.Sprintf("phase-%04d.json", f.sequence)), report)
	verifyBlackout(t, report)
	verifyOriginReceive(t, report)
	assertResults(t, state.Policy, report.Flows)
	t.Logf("cycle=%d phase=%s workers=%d UDP_packets=%d duration_ms=%d", cycle, state.Name, workers, packets, report.DurationMS)
	return report.Flows
}

func (f *fixture) recover(t *testing.T, cycle int, label string) {
	t.Helper()
	state := impairment{Name: "recovery-" + label, MTU: 1500, Netem: []string{"delay", "0ms", "loss", "0%"}, Policy: "observe"}
	shape(t, state)
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		flows := f.phase(t, cycle, state, 1, 1)
		if complete(flows) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("clean-link recovery exceeded 45 seconds: %s", label)
}

func complete(flows []flowResult) bool {
	for _, flow := range flows {
		if flow.Sent == 0 || flow.Sent != flow.Received || flow.Error != "" || flow.Foreign != 0 {
			return false
		}
	}
	return len(flows) != 0
}

func assertResults(t *testing.T, policy string, flows []flowResult) {
	t.Helper()
	for _, flow := range flows {
		if flow.Foreign != 0 || flow.Attempted == 0 || policy == "blocked" && flow.Received != 0 {
			t.Errorf("unexpected flow result under %s: %+v", policy, flow)
		}
	}
	if policy == "complete" && !complete(flows) {
		t.Errorf("loss-free phase did not complete: %+v", flows)
	}
}
