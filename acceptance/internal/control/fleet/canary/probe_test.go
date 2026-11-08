package canary

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"vpn-script/internal/control/integration/traffic"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
)

func TestFleetGuestTraffic(t *testing.T) {
	root := guestRoot(t)
	var input probeInput
	require(t, safeio.LoadJSON(filepath.Join(root, "probe.json"), &input))
	var endpoints []traffic.Endpoint
	for _, client := range input.Clients {
		startProbe(t, root, input.Phase, client)
		endpoints = append(endpoints, traffic.Endpoint{ID: client.ID, Port: client.Port})
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	report, err := traffic.Run(ctx, endpoints, 5, 20*time.Millisecond)
	report.Phase = input.Phase
	require(t, safeio.NewJSON(filepath.Join(root, input.Phase+"-traffic.json"), report))
	if err != nil {
		observeRecovery(t, root, input.Phase, endpoints)
	}
	require(t, err)
	if report.Profiles != 2 || report.BarrierRounds != 5 || report.ConcurrentStreams != 4 {
		t.Fatal("both three-hop protocols must cross every TCP/UDP barrier")
	}
}

// Recovery is diagnostic only: the first failed barrier still fails the test.
// Keep all engines running to distinguish reconnect delay from a server restart.
func observeRecovery(t *testing.T, root, phase string, endpoints []traffic.Endpoint) {
	t.Helper()
	var observations []traffic.Report
	for attempt := 0; attempt < 3; attempt++ {
		time.Sleep(2 * time.Second)
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		report, err := traffic.Run(ctx, endpoints, 5, 20*time.Millisecond)
		cancel()
		report.Phase = phase + "-recovery-" + strconv.Itoa(attempt+1)
		observations = append(observations, report)
		if err == nil {
			break
		}
	}
	require(t, safeio.NewJSON(filepath.Join(root, phase+"-recovery.json"), observations))
}

func startProbe(t *testing.T, root, phase string, client probeClient) {
	t.Helper()
	config := filepath.Join(root, phase+"-"+client.ID+".json")
	require(t, safeio.NewJSON(config, client.Config))
	log, err := os.OpenFile(config+".log", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	require(t, err)
	cmd := process.Command(context.Background(), filepath.Join(nodeBase, "current", client.Engine), engines.Arguments(client.Engine, config, false)...)
	cmd.Stdout, cmd.Stderr = log, log
	require(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		process.Stop(cmd)
		<-done
		require(t, log.Close())
	})
	eventually(t, "probe SOCKS listener", 10*time.Second, func() bool {
		connection, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(client.Port)), 200*time.Millisecond)
		if err != nil {
			return false
		}
		connection.Close()
		return true
	})
}
