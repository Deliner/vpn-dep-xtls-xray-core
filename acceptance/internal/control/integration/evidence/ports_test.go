package evidence

import (
	"reflect"
	"testing"
)

func TestPortRangesRejectMissingAndMalformedKernelEvidence(t *testing.T) {
	for _, raw := range []string{"", "49152", "49152 65535 extra", "49152 nope", "0 65535", "1 65536", "-1 60000"} {
		if _, err := parsePortRanges(raw, 1); err == nil {
			t.Errorf("accepted ambiguous port range %q", raw)
		}
	}
	if _, err := parsePortRanges("49152 65535", 2); err == nil {
		t.Fatal("missing Darwin high range accepted")
	}
	actual, err := parsePortRanges("65535\n49152\n60000\n61000\n", 2)
	if err != nil || !reflect.DeepEqual(actual, [][2]int{{49152, 65535}, {60000, 61000}}) {
		t.Fatalf("descending or independent high range lost: %v, %v", actual, err)
	}
}

func TestListenerPoolExcludesEveryAutomaticRangeInclusively(t *testing.T) {
	ports, err := listenerCandidates([][2]int{{10000, 15000}, {15500, 19999}})
	if err != nil || len(ports) != 499 || ports[0] != 15001 || ports[len(ports)-1] != 15499 {
		t.Fatalf("listener pool intersects an automatic range: %v, %v", ports, err)
	}
	for index, port := range ports {
		if port != 15001+index {
			t.Fatal("duplicate, missing or unordered candidate")
		}
	}
	if _, err := listenerCandidates([][2]int{{1, 65535}}); err == nil {
		t.Fatal("exhausted pool silently fell back to automatic allocation")
	}
}

func TestListenerPoolObservesTheCurrentKernel(t *testing.T) {
	ports, err := ListenerPorts()
	if err != nil || len(ports) == 0 {
		t.Fatal("could not observe a disjoint listener pool", err)
	}
	if ports[0] < 10000 || ports[len(ports)-1] >= 20000 {
		t.Fatal("listener pool overlaps privileged ports or internal bridges")
	}
}
