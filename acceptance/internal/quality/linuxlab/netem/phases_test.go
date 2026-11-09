package netem

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func successfulFlows(name string) phase {
	result := phase{Phase: impairment{Name: name, Policy: "complete", MTU: 1500}}
	for worker := range 8 {
		result.Flows = append(result.Flows, flow{Network: "tcp", Worker: worker, Attempted: 1, Sent: 1, Received: 1},
			flow{Network: "udp", Worker: worker, Attempted: 32, Sent: 32, Received: 32})
	}
	return result
}

func TestControlTimeoutCannotExcuseRenoOrOtherPhaseFailure(t *testing.T) {
	for _, policy := range []string{"default", "reno"} {
		for _, name := range []string{"delay", "baseline", "mtu-1280", "stable-after-client"} {
			phase := successfulFlows(name)
			phase.Flows[0].Received, phase.Flows[0].Error = 0, "read tcp: i/o timeout"
			result := Result{Policy: policy}
			complete, err := evaluateFlows(phase, &result)
			control := policy == "default" && name == "delay"
			if complete || (err == nil) != control || control && result.DelayFailures != 1 {
				t.Fatalf("control failure classification changed: %s/%s: %+v %v", policy, name, result, err)
			}
		}
	}
}

func TestObservedLossRetainsSocketClosureInsteadOfInventingItsCause(t *testing.T) {
	phase := successfulFlows("loss")
	phase.Phase.Policy = "observe"
	phase.Flows[1].Received, phase.Flows[1].Missing = 31, []int{31}
	phase.Flows[1].Error = "read udp4: use of closed network connection"
	complete, err := evaluateFlows(phase, &Result{Policy: "reno"})
	if err != nil || complete {
		t.Fatalf("observed loss was rejected or reported as successful delivery: %v", err)
	}
}

func TestFlowEvidenceRejectsUnprovenCoverageAndLoss(t *testing.T) {
	for _, damage := range []string{"valid", "missing-worker", "duplicate-worker", "foreign", "duplicates", "wrong-attempts", "extra-replies", "silent-loss", "false-timeout", "unexplained-missing"} {
		t.Run(damage, func(t *testing.T) {
			phase := damagedFlows(damage)
			complete, err := evaluateFlows(phase, &Result{Policy: "reno"})
			if (err == nil && complete) != (damage == "valid") {
				t.Fatalf("invalid coverage accepted: %s: %v", damage, err)
			}
		})
	}
}

func damagedFlows(kind string) phase {
	phase := successfulFlows("delay")
	switch kind {
	case "missing-worker":
		phase.Flows = phase.Flows[1:]
	case "duplicate-worker":
		phase.Flows[0] = phase.Flows[2]
	case "foreign":
		phase.Flows[1].Foreign = 1
	case "duplicates":
		phase.Flows[1].Duplicate = 1
	case "wrong-attempts":
		phase.Flows[1].Attempted = 31
	case "extra-replies":
		phase.Flows[1].Received = 33
	case "silent-loss":
		phase.Flows[0].Received = 0
	case "false-timeout":
		phase.Flows[0].Error = "i/o timeout"
	case "unexplained-missing":
		phase.Flows[1].Received, phase.Flows[1].Error = 31, "i/o timeout"
	}
	return phase
}

func TestNetworkReadbackRejectsWrongDelayMTUAndHiddenOverflow(t *testing.T) {
	for _, damage := range []string{"valid", "delay", "mtu", "origin", "drops", "clock"} {
		phase := observedPhase(t)
		switch damage {
		case "delay":
			phase.Before.Client.Queue = []byte(strings.Replace(string(phase.Before.Client.Queue), `"delay":0.02`, `"delay":0`, 1))
		case "mtu":
			phase.Before.Exit.Interface = []byte(`[{"mtu":1280}]`)
		case "origin":
			phase.After.Exit.UDPSockets = strings.Replace(phase.After.Exit.UDPSockets, " 77", " 78", 1)
		case "drops":
			phase.Before.Client.Drops = 99
		case "clock":
			phase.Finished = phase.Started
		}
		if err := phaseObservation(phase); (err == nil) != (damage == "valid") {
			t.Fatalf("readback accepted %s: %v", damage, err)
		}
	}
}

func observedPhase(t *testing.T) phase {
	t.Helper()
	queue := `[{"kind":"netem","handle":"10:","root":true,"drops":0,"options":{"limit":4096,"delay":{"delay":0.02,"jitter":0.005,"correlation":0},"ecn":false,"gap":0}}]`
	link := link{Queue: json.RawMessage(queue), Interface: json.RawMessage(`[{"mtu":1500}]`),
		UDPSockets: "1067: 020012C6:46A1 00000000:0000 07 00000000:00000000 00:00000000 00000000 0 0 6976 2 pointer 77"}
	return phase{Phase: impairment{Name: "delay", MTU: 1500}, Started: time.Unix(1, 0), Finished: time.Unix(2, 0), DurationMS: 1000,
		Before: observation{Client: link, Exit: link}, After: observation{Client: link, Exit: link}}
}
