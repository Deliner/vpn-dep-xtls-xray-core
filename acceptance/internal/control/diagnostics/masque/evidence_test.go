package masque

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/platform/safeio"
)

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := safeio.JSON(path, value); err != nil {
		t.Fatal(err)
	}
}

func attributeQueueDrops(t *testing.T, directory string) {
	t.Helper()
	switch kind := os.Getenv("NHOP_MASQUE_RUNTIME_KIND"); kind {
	case "runtime-candidate":
		writeJSON(t, filepath.Join(directory, "queue-attribution.json"), map[string]any{
			"available": false, "reason": "runtime-candidate has no diagnostic queue observer",
		})
		t.Log("HTTP/3 queue observations unavailable for ordinary runtime candidate")
		return
	case "diagnostic-probe":
	default:
		t.Fatalf("unknown queue observation source: %q", kind)
	}
	report := correlateQueueEvidence(t, directory)
	writeJSON(t, filepath.Join(directory, "queue-attribution.json"), report)
	t.Logf("HTTP/3 queue drops: total=%d UDP=%d TCP=%d; packet/time attribution saved", report.Total, report.UDP, report.TCP)
}

func correlateQueueEvidence(t *testing.T, directory string) queueEvidence {
	t.Helper()
	events := readQueueDrops(t, directory)
	report := queueEvidence{Available: true, Total: len(events)}
	byID := map[packetID][]int64{}
	for _, event := range events {
		if event.Protocol == 17 {
			report.UDP++
		} else if event.Protocol == 6 {
			report.TCP++
		}
		if event.HasID {
			byID[event.ID] = append(byID[event.ID], event.UnixNS)
		}
	}
	paths, err := filepath.Glob(filepath.Join(directory, "phase-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		var phase phaseReport
		if err := json.Unmarshal(readEvidence(t, path), &phase); err != nil {
			t.Fatal(err)
		}
		report.Phases = append(report.Phases, correlatePhase(phase, byID))
	}
	return report
}

func TestQueueAttributionDistinguishesUnavailableFromMeasuredZero(t *testing.T) {
	for _, kind := range []string{"runtime-candidate", "diagnostic-probe"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("NHOP_MASQUE_RUNTIME_KIND", kind)
			directory := t.TempDir()
			attributeQueueDrops(t, directory)
			var result map[string]any
			if err := json.Unmarshal(readEvidence(t, filepath.Join(directory, "queue-attribution.json")), &result); err != nil {
				t.Fatal(err)
			}
			available := kind == "diagnostic-probe"
			if result["available"] != available {
				t.Fatal("queue observation availability changed")
			}
			for _, field := range []string{"queue_events", "udp_events", "tcp_events", "phases"} {
				if _, present := result[field]; present != available {
					t.Fatalf("queue field %s invented or omitted for %s", field, kind)
				}
			}
			if available && result["queue_events"] != float64(0) {
				t.Fatal("an empty observed queue report has nonzero events")
			}
		})
	}
}

func readQueueDrops(t *testing.T, directory string) []queueDrop {
	t.Helper()
	var events []queueDrop
	for _, role := range []string{"client", "server"} {
		paths, err := filepath.Glob(filepath.Join(directory, role+"-*.log"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range paths {
			for _, line := range strings.Split(string(readEvidence(t, path)), "\n") {
				if !strings.HasPrefix(line, "NHOP_H3_QUEUE_DROP ") {
					continue
				}
				event, err := parseQueueDrop(line)
				if err != nil {
					t.Fatal(path, err)
				}
				events = append(events, event)
			}
		}
	}
	return events
}

func correlatePhase(phase phaseReport, events map[packetID][]int64) phaseAttribution {
	result := phaseAttribution{Nonce: phase.Nonce, Cycle: phase.Cycle, Name: phase.Phase.Name}
	start := phase.Started.UnixNano()
	end := phase.Finished.UnixNano()
	for _, flow := range phase.Flows {
		if flow.Network != "udp" {
			continue
		}
		for _, sequence := range flow.Missing {
			id := packetID{uint64(phase.Nonce), uint32(flow.Worker), uint32(sequence)}
			result.Missing++
			switch dropWindow(events[id], start, end) {
			case "during":
				result.During++
			case "late":
				result.Late++
			default:
				result.Unexplained++
			}
		}
	}
	return result
}

func dropWindow(events []int64, start, end int64) string {
	result := "absent"
	for _, at := range events {
		if at >= start && at <= end {
			return "during"
		}
		if at > end {
			result = "late"
		}
	}
	return result
}

func readEvidence(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestQueueCorrelationSeparatesLateAndUnexplainedLoss(t *testing.T) {
	phase := phaseReport{Nonce: 7, Started: time.Unix(0, 100), Finished: time.Unix(0, 1_000_100), DurationMS: 1,
		Flows: []flowResult{{Network: "udp", Worker: 2, Missing: []int{0, 1, 2}}}}
	result := correlatePhase(phase, map[packetID][]int64{
		{7, 2, 0}: {200}, {7, 2, 1}: {2_000_000}, {8, 2, 2}: {200},
	})
	if result.Missing != 3 || result.During != 1 || result.Late != 1 || result.Unexplained != 1 {
		t.Fatalf("late or different-phase loss was misattributed: %+v", result)
	}
}
