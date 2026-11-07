package traffic

import (
	"fmt"
	"sort"
)

func newBarrier(endpoints []Endpoint, count int) (*barrier, error) {
	if len(endpoints) == 0 || count < 1 {
		return nil, fmt.Errorf("nonempty endpoints and positive round count required")
	}
	b := &barrier{expected: map[string]bool{}}
	for _, endpoint := range endpoints {
		if endpoint.ID == "" || len(endpoint.ID) > 200 || b.expected[endpoint.ID] {
			return nil, fmt.Errorf("empty, oversized or duplicate route ID")
		}
		b.expected[endpoint.ID] = true
	}
	for range count {
		b.rounds = append(b.rounds, &round{seen: map[string]bool{}, ready: make(chan struct{})})
	}
	return b, nil
}

func (b *barrier) arrive(id string, sequence int, network string) (<-chan struct{}, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.expected[id] || sequence < 0 || sequence >= len(b.rounds) || (network != "tcp" && network != "udp") {
		return nil, fmt.Errorf("unexpected route, round or network")
	}
	round := b.rounds[sequence]
	key := id + "/" + network
	if round.seen[key] {
		return nil, fmt.Errorf("duplicate arrival for %s round %d", key, sequence)
	}
	round.seen[key] = true
	b.peak = max(b.peak, len(round.seen))
	if len(round.seen) == 2*len(b.expected) {
		b.completed++
		close(round.ready)
	}
	return round.ready, nil
}

func (b *barrier) snapshot(report *Report) {
	b.mu.Lock()
	defer b.mu.Unlock()
	report.BarrierRounds, report.ConcurrentStreams = b.completed, b.peak
	for index, round := range b.rounds {
		if len(round.seen) == 2*len(b.expected) {
			continue
		}
		for id := range b.expected {
			for _, network := range []string{"tcp", "udp"} {
				if !round.seen[id+"/"+network] {
					report.Missing = append(report.Missing, fmt.Sprintf("round %d: %s/%s", index, id, network))
				}
			}
		}
		break
	}
	sort.Strings(report.Missing)
}
