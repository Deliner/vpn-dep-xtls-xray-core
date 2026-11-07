package traffic

import (
	"context"
	"fmt"
	"net"
	"time"
)

func Run(ctx context.Context, endpoints []Endpoint, rounds int, interval time.Duration) (Report, error) {
	started := time.Now()
	report := Report{Profiles: len(endpoints), RequiredRounds: rounds, TCPBytes: tcpBytes, UDPBytes: udpBytes, Routes: map[string]Result{}}
	gate, err := newBarrier(endpoints, rounds)
	if err != nil {
		return report, err
	}
	o, err := startOrigin(ctx, gate)
	if err != nil {
		return report, err
	}
	defer o.close()
	done := make(chan outcome, len(endpoints))
	for _, endpoint := range endpoints {
		go send(o, endpoint, rounds, interval, done)
	}
	for range endpoints {
		outcome := <-done
		report.Routes[outcome.id] = outcome.result
		if err == nil && outcome.result.Error != "" {
			err = fmt.Errorf("%s: %s", outcome.id, outcome.result.Error)
			o.cancel()
		}
	}
	gate.snapshot(&report)
	report.DurationMS = time.Since(started).Milliseconds()
	if err == nil && report.BarrierRounds != rounds {
		err = fmt.Errorf("not all routes crossed every TCP/UDP barrier")
	}
	return report, err
}

func send(o *origin, endpoint Endpoint, rounds int, interval time.Duration, done chan<- outcome) {
	result := Result{}
	session, err := openSession(o.context, endpoint, o.tcp.Addr().(*net.TCPAddr).Port)
	if err == nil {
		defer session.close()
		err = transfer(o.context, session, endpoint.ID, o.udp.LocalAddr().(*net.UDPAddr).Port, rounds, interval, &result)
	}
	if err != nil {
		result.Error = err.Error()
	}
	done <- outcome{id: endpoint.ID, result: result}
}

func transfer(ctx context.Context, session *session, id string, target, rounds int, interval time.Duration, result *Result) error {
	for sequence := range rounds {
		if err := session.exchange(id, sequence, target, result); err != nil {
			return fmt.Errorf("round %d: %w", sequence, err)
		}
		if sequence+1 == rounds {
			break
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}
