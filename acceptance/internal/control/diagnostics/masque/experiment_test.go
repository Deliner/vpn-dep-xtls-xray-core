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

// Delay history is a diagnostic, not a replacement for the strict fault matrix.
// Warm windows retain failures as observations; fresh delayed tunnels and clean
// baselines must still complete under the unchanged traffic deadline.
func TestMASQUEDelayHistory(t *testing.T) {
	if os.Getenv("NHOP_MASQUE_NETEM_TEST") != "private-netns" || os.Getenv("NHOP_MASQUE_SCENARIO") != "delay-history" {
		t.Skip("requires the owned Linux delay-history experiment")
	}
	f := setup(t)
	cycles := boundedEnvironment(t, "NHOP_MASQUE_CYCLES", 2, 20)
	workers := boundedEnvironment(t, "NHOP_MASQUE_WORKERS", 8, 64)
	packets := boundedEnvironment(t, "NHOP_MASQUE_PACKETS", 32, 256)
	for cycle := range cycles {
		if cycle%2 == 0 {
			f.delayWarm(t, cycle, workers, packets)
			f.delayCold(t, cycle, workers, packets)
		} else {
			f.delayCold(t, cycle, workers, packets)
			f.delayWarm(t, cycle, workers, packets)
		}
		f.recover(t, cycle, "delay-history-finished")
	}
}

func (f *fixture) delayWarm(t *testing.T, cycle, workers, packets int) {
	t.Helper()
	clean := impairment{Name: "warm-baseline", MTU: 1500, Netem: []string{"delay", "0ms", "loss", "0%"}, Policy: "complete"}
	clean.Traffic = historyTraffic(t)
	shape(t, clean)
	f.restart(t, "server")
	f.restart(t, "client")
	f.recover(t, cycle, "warm-history")
	f.phase(t, cycle, clean, workers, packets)
	delay := historyDelay(t, "", "observe")
	shape(t, delay)
	step := time.Now()
	for window := range 7 {
		if remaining := time.Until(step.Add(time.Duration(window) * 2 * time.Second)); remaining > 0 {
			time.Sleep(remaining)
		}
		delay.Name = fmt.Sprintf("warm-delay-%02d", window*2)
		f.historyIdentity(t, cycle, delay.Name, step)
		f.phase(t, cycle, delay, workers, packets)
	}
}

func (f *fixture) delayCold(t *testing.T, cycle, workers, packets int) {
	t.Helper()
	delay := historyDelay(t, "cold-delay", "complete")
	shape(t, delay)
	f.restart(t, "server")
	f.restart(t, "client")
	f.historyIdentity(t, cycle, delay.Name, time.Now())
	f.phase(t, cycle, delay, workers, packets)
}

func (f *fixture) historyIdentity(t *testing.T, cycle int, phase string, step time.Time) {
	t.Helper()
	writeJSON(t, filepath.Join(f.directory, fmt.Sprintf("history-%04d.json", f.sequence+1)), map[string]any{
		"scenario": "delay-history", "cycle": cycle, "phase": phase, "nonce": f.sequence + 1,
		"step_started": step.UTC(), "observed": time.Now().UTC(),
		"client_pid": f.client.command.Process.Pid, "server_pid": f.server.command.Process.Pid,
		"deadline_ms": 2000, "warm_windows": 7, "spacing_ms": 2000,
		"jitter_ms": os.Getenv("NHOP_MASQUE_DELAY_JITTER_MS"),
		"queue":     os.Getenv("NHOP_MASQUE_DELAY_QUEUE"),
		"traffic":   historyTraffic(t),
	})
}

func historyDelay(t *testing.T, name, policy string) impairment {
	t.Helper()
	jitter := os.Getenv("NHOP_MASQUE_DELAY_JITTER_MS")
	if jitter != "0" && jitter != "5" {
		t.Fatal("delay-history jitter must be explicitly 0 or 5 milliseconds")
	}
	return impairment{Name: name, MTU: 1500, Netem: []string{"delay", "20ms", jitter + "ms", "loss", "0%"}, Policy: policy, Traffic: historyTraffic(t)}
}

func historyTraffic(t *testing.T) string {
	t.Helper()
	traffic := os.Getenv("NHOP_MASQUE_DELAY_TRAFFIC")
	if _, err := trafficNetworks(traffic); err != nil || traffic == "" {
		t.Fatal("delay-history traffic must be explicitly mixed, tcp or udp")
	}
	return traffic
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
	networks, err := trafficNetworks(state.Traffic)
	if err != nil {
		t.Fatal(err)
	}
	f.sequence++
	report := phaseReport{Nonce: f.sequence, Phase: state, Cycle: cycle, Started: time.Now().UTC()}
	report.Before = observeNetwork(t, state.MTU)
	report.Flows = measure(workers, packets, f.sequence, 2*time.Second, networks)
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
