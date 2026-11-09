package masque

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"testing"
	"time"

	"vpn-script/internal/control/integration/traffic"
)

func measure(workers, packets, nonce int, window time.Duration, networks []string) []flowResult {
	ctx, cancel := context.WithTimeout(context.Background(), window)
	defer cancel()
	count := workers * len(networks)
	ready, begin := make(chan struct{}, count), make(chan struct{})
	finished := make(chan flowResult, count)
	for worker := range workers {
		for _, network := range networks {
			go func() {
				ready <- struct{}{}
				<-begin
				if network == "udp" {
					finished <- udpBurst(ctx, worker, packets, nonce)
				} else {
					finished <- tcpPayload(ctx, worker, nonce)
				}
			}()
		}
	}
	for range count {
		<-ready
	}
	close(begin)
	var results []flowResult
	for range count {
		results = append(results, <-finished)
	}
	return results
}

func trafficNetworks(selection string) ([]string, error) {
	switch selection {
	case "", "mixed":
		return []string{"udp", "tcp"}, nil
	case "udp", "tcp":
		return []string{selection}, nil
	default:
		return nil, fmt.Errorf("unknown traffic selection: %q", selection)
	}
}

func TestTrafficSelectionRejectsUnknownOrPartialNames(t *testing.T) {
	for _, invalid := range []string{"TCP", "tcp,udp", "none", " tcp", "udp ", "tcp\x00"} {
		if networks, err := trafficNetworks(invalid); err == nil || len(networks) != 0 {
			t.Fatalf("invalid selection could silently change the measured workload: %q", invalid)
		}
	}
}

func packet(nonce, worker, sequence, size int) []byte {
	value := bytes.Repeat([]byte{0x5a}, size)
	binary.BigEndian.PutUint64(value[:8], uint64(nonce))
	binary.BigEndian.PutUint32(value[8:12], uint32(worker))
	binary.BigEndian.PutUint32(value[12:16], uint32(sequence))
	return value
}

func udpBurst(ctx context.Context, worker, packets, nonce int) flowResult {
	result := flowResult{Network: "udp", Worker: worker, Attempted: packets}
	connection, err := traffic.OpenUDPProbe(ctx, socksPort)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer connection.Close()
	deadline, _ := ctx.Deadline()
	expected := map[string][]byte{}
	for sequence := range packets {
		data := packet(nonce, worker, sequence, 900)
		if err := connection.Send(udpOriginPort, data, deadline, [4]byte{198, 18, 0, 2}); err != nil {
			result.Error = err.Error()
			return result
		}
		expected[string(data[:16])] = data
		result.Sent++
	}
	receiveBurst(connection, deadline, expected, &result)
	return result
}

func receiveBurst(connection *traffic.UDPProbe, deadline time.Time, expected map[string][]byte, result *flowResult) {
	seen := map[string]bool{}
	defer func() {
		for key := range expected {
			if !seen[key] {
				result.Missing = append(result.Missing, int(binary.BigEndian.Uint32([]byte(key)[12:])))
			}
		}
		sort.Ints(result.Missing)
	}()
	for result.Received < result.Sent {
		data, err := connection.Receive(udpOriginPort, deadline, [4]byte{198, 18, 0, 2})
		if err != nil {
			result.Error = err.Error()
			return
		}
		key := string(data[:min(16, len(data))])
		if want, found := expected[key]; !found || !bytes.Equal(data, want) {
			result.Foreign++
		} else if seen[key] {
			result.Duplicate++
		} else {
			seen[key] = true
			result.Received++
		}
	}
}

func tcpPayload(ctx context.Context, worker, nonce int) flowResult {
	result := flowResult{Network: "tcp", Worker: worker, Attempted: 1}
	data := packet(nonce, worker, 0, 32768)
	sent, err := tcpExchange(ctx, data)
	result.Sent = sent
	if err == nil {
		result.Received = 1
	} else {
		result.Error = err.Error()
		if errors.Is(err, errPayload) {
			result.Foreign = 1
		}
	}
	return result
}

var errPayload = fmt.Errorf("echoed TCP payload differs")

func tcpExchange(ctx context.Context, data []byte) (int, error) {
	connection, err := traffic.OpenTCPProbe(ctx, socksPort, 18080)
	if err != nil {
		return 0, err
	}
	defer connection.Close()
	deadline, _ := ctx.Deadline()
	if err := connection.SetDeadline(deadline); err != nil {
		return 0, err
	}
	if _, err := io.Copy(connection, bytes.NewReader(data)); err != nil {
		return 0, err
	}
	reply := make([]byte, len(data))
	if _, err := io.ReadFull(connection, reply); err != nil {
		return 1, err
	}
	if !bytes.Equal(reply, data) {
		return 1, errPayload
	}
	return 1, nil
}

func TestPayloadIdentitySeparatesPhasesWorkersAndPackets(t *testing.T) {
	seen := map[string]bool{}
	for phase := range 2 {
		for worker := range 3 {
			for sequence := range 4 {
				key := string(packet(phase, worker, sequence, 900)[:16])
				if seen[key] {
					t.Fatal("late or cross-worker reply could be accepted as a new packet")
				}
				seen[key] = true
			}
		}
	}
}
