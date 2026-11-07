package traffic

import (
	"context"
	"testing"
)

func TestBarrierRequiresEveryRouteAndNetwork(t *testing.T) {
	b, err := newBarrier([]Endpoint{{ID: "alpha"}, {ID: "beta"}}, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range [][2]string{{"alpha", "tcp"}, {"alpha", "udp"}, {"beta", "tcp"}} {
		ready, err := b.arrive(value[0], 0, value[1])
		if err != nil {
			t.Fatal(err)
		}
		assertPending(t, ready)
	}
	if _, err := b.arrive("alpha", 0, "udp"); err == nil {
		t.Fatal("duplicate could replace the missing route")
	}
	ready, err := b.arrive("beta", 0, "udp")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ready:
	default:
		t.Fatal("complete barrier did not release")
	}
}

func TestRoundsDoNotSatisfyEachOther(t *testing.T) {
	b, err := newBarrier([]Endpoint{{ID: "alpha"}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	first, err := b.arrive("alpha", 0, "tcp")
	if err != nil {
		t.Fatal(err)
	}
	second, err := b.arrive("alpha", 1, "udp")
	if err != nil {
		t.Fatal(err)
	}
	assertPending(t, first)
	assertPending(t, second)
	var report Report
	b.snapshot(&report)
	if report.BarrierRounds != 0 || len(report.Missing) != 1 || report.Missing[0] != "round 0: alpha/udp" {
		t.Fatalf("incomplete round incorrectly assessed: %+v", report)
	}
}

func TestCancellationCannotPassEmptyTraffic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := Run(ctx, []Endpoint{{ID: "unreachable", Port: 1}}, 25, 0)
	if err == nil || report.BarrierRounds != 0 || len(report.Missing) != 2 {
		t.Fatalf("cancellation reported success: %+v, %v", report, err)
	}
}

func assertPending(t *testing.T, ready <-chan struct{}) {
	t.Helper()
	select {
	case <-ready:
		t.Fatal("incomplete TCP/UDP barrier released")
	default:
	}
}
