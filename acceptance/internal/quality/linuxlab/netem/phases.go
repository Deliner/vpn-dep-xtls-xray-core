package netem

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

func expectedPhases() []impairment {
	states := []impairment{{Name: "recovery-initial", MTU: 1500, Netem: []string{"delay", "0ms", "loss", "0%"}, Policy: "observe"}}
	for _, state := range []impairment{
		{Name: "baseline", MTU: 1500, Netem: []string{"delay", "0ms", "loss", "0%"}, Policy: "complete"},
		{Name: "delay", MTU: 1500, Netem: []string{"delay", "20ms", "5ms", "loss", "0%"}, Policy: "complete"},
		{Name: "loss", MTU: 1500, Netem: []string{"delay", "10ms", "loss", "2%"}, Policy: "observe"},
		{Name: "mtu-1400", MTU: 1400, Netem: []string{"delay", "5ms", "loss", "0%"}, Policy: "complete"},
		{Name: "mtu-1280", MTU: 1280, Netem: []string{"delay", "5ms", "loss", "0%"}, Policy: "complete"},
		{Name: "blackout", MTU: 1500, Netem: []string{"loss", "100%"}, Policy: "blocked"},
	} {
		states = append(states, state, impairment{Name: "recovery-after-" + state.Name, MTU: 1500,
			Netem: []string{"delay", "0ms", "loss", "0%"}, Policy: "observe"})
	}
	for _, role := range []string{"client", "server"} {
		states = append(states, impairment{Name: "recovery-restart-" + role, MTU: 1500,
			Netem: []string{"delay", "0ms", "loss", "0%"}, Policy: "observe"},
			impairment{Name: "stable-after-" + role, MTU: 1500, Policy: "complete"})
	}
	return states
}

func verifyPhases(root, policy string) (Result, error) {
	result := Result{Policy: policy}
	paths, err := filepath.Glob(filepath.Join(root, "phase-*.json"))
	if err != nil {
		return result, err
	}
	states, next, lastNonce := expectedPhases(), 0, 0
	for _, path := range paths {
		var phase phase
		if err := readJSON(path, &phase); err != nil {
			return result, err
		}
		if next >= 8*len(states) || phase.Nonce <= lastNonce || phase.Cycle != next/len(states) || !reflect.DeepEqual(phase.Phase, states[next%len(states)]) {
			return result, fmt.Errorf("missing, duplicate or changed network phase: %s", path)
		}
		if err := phaseObservation(phase); err != nil {
			return result, fmt.Errorf("%s: %w", path, err)
		}
		complete, err := evaluateFlows(phase, &result)
		if err != nil {
			return result, fmt.Errorf("%s: %w", path, err)
		}
		if !strings.HasPrefix(phase.Phase.Name, "recovery-") || complete {
			next++
		}
		lastNonce = phase.Nonce
	}
	if next != 8*len(states) {
		return result, fmt.Errorf("incomplete eight-cycle network experiment")
	}
	return result, nil
}

func evaluateFlows(phase phase, result *Result) (bool, error) {
	workers, packets := 8, 32
	if strings.HasPrefix(phase.Phase.Name, "recovery-") {
		workers, packets = 1, 1
	}
	if len(phase.Flows) != workers*2 {
		return false, fmt.Errorf("missing TCP/UDP workers")
	}
	seen, complete := map[string]bool{}, true
	for _, flow := range phase.Flows {
		key := fmt.Sprintf("%s/%d", flow.Network, flow.Worker)
		if seen[key] || flow.Worker < 0 || flow.Worker >= workers {
			return false, fmt.Errorf("invalid or duplicate flow: %s", key)
		}
		seen[key] = true
		if err := evaluateFlow(flow, phase.Phase.Policy, packets, result); err != nil {
			return false, err
		}
		complete = complete && flow.Received == flow.Attempted && flow.Error == ""
	}
	if phase.Phase.Policy == "complete" && !complete {
		if result.Policy != "default" || phase.Phase.Name != "delay" {
			return false, fmt.Errorf("loss-free phase failed under %s: %s", result.Policy, phase.Phase.Name)
		}
		result.DelayFailures++
	}
	return complete, nil
}

func evaluateFlow(flow flow, policy string, packets int, result *Result) error {
	want := packets
	if flow.Network == "tcp" {
		want = 1
	} else if flow.Network != "udp" {
		return fmt.Errorf("unknown flow network")
	}
	if flow.Attempted != want || flow.Sent < 0 || flow.Sent > want || flow.Received < 0 || flow.Received > flow.Sent || flow.Duplicate != 0 || flow.Foreign != 0 {
		return fmt.Errorf("invalid delivery counters")
	}
	if policy == "blocked" && flow.Received != 0 {
		return fmt.Errorf("blackout delivered application traffic")
	}
	if flow.Received < want && flow.Error == "" {
		return fmt.Errorf("incomplete flow without a recorded error")
	}
	if flow.Network == "udp" {
		seen := map[int]bool{}
		for _, sequence := range flow.Missing {
			if sequence < 0 || sequence >= flow.Sent || seen[sequence] {
				return fmt.Errorf("invalid missing datagram identity")
			}
			seen[sequence] = true
		}
		if len(seen) != flow.Sent-flow.Received {
			return fmt.Errorf("missing datagram count differs")
		}
	}
	if policy == "complete" && flow.Network == "tcp" {
		result.TCP, result.ExpectedTCP = result.TCP+flow.Received, result.ExpectedTCP+want
	} else if policy == "complete" {
		result.UDP, result.ExpectedUDP = result.UDP+flow.Received, result.ExpectedUDP+want
	}
	return nil
}
