package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"
	"vpn-script/internal/control/client"
	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/control/integration/traffic"
	"vpn-script/internal/core/dag"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
)

func TestProtocolMatrixPayload(t *testing.T) {
	for _, hops := range []int{1, 3} {
		t.Run(map[int]string{1: "single", 3: "mixed-three-hop"}[hops], func(t *testing.T) {
			l := matrixLab(t, hops)
			for _, name := range dag.Keys(l.topology.Routes) {
				t.Run(name, func(t *testing.T) {
					config, profile := engines.Client(l.topology, name, l.secrets)
					stop := l.startClient(t, "client-"+name, profile.Engine, config)
					defer stop()
					require(t, waitPort(profile.SOCKSPort))
					body, err := l.http(profile.SOCKSPort)
					require(t, err)
					if !bytes.Equal(body, payload) {
						t.Fatal("TCP payload mismatch")
					}
					packet := bytes.Repeat([]byte{0x51}, 512)
					got, err := traffic.UDPRoundTrip(profile.SOCKSPort, l.origin.udp.LocalAddr().(*net.UDPAddr).Port, packet, [4]byte{198, 18, 0, 2})
					require(t, err)
					if !bytes.Equal(packet, got) {
						t.Fatal("UDP payload mismatch")
					}
				})
			}
		})
	}
}

func TestGeneratedClientLauncher(t *testing.T) {
	l := matrixLab(t, 1)
	for _, name := range []string{"trojan", "tuic"} {
		t.Run(name, func(t *testing.T) {
			config, profile := engines.Client(l.topology, name, l.secrets)
			path := filepath.Join(l.directory, "launcher-"+name+".json")
			require(t, safeio.JSON(path, config))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			binaries := map[string]string{"xray": l.clientBinary("xray"), "sing-box": l.clientBinary("sing-box")}
			go func() { done <- client.Run(ctx, path, binaries, io.Discard) }()
			require(t, waitPort(profile.SOCKSPort))
			body, err := l.http(profile.SOCKSPort)
			require(t, err)
			if !bytes.Equal(body, payload) {
				t.Fatal("launcher TCP mismatch")
			}
			cancel()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("client did not stop")
			}
		})
	}
}

func TestAllProtocolsConcurrent(t *testing.T) {
	for _, hops := range []int{1, 3} {
		t.Run(fmt.Sprintf("%d-hop", hops), func(t *testing.T) { concurrentScenario(t, hops) })
	}
}

func concurrentScenario(t *testing.T, hops int, addresses ...string) {
	report := concurrentSuite{SchemaVersion: 4, Hops: hops, Scope: "native runtimes on explicit node IPs; IPv4 SOCKS, internal bridges and origins; no VPS deployment or DPI test",
		StartedAt: time.Now().UTC().Format(time.RFC3339), Paths: map[string][]string{},
		Platform: runtime.GOOS + "/" + runtime.GOARCH, TestGoVersion: runtime.Version()}
	t.Cleanup(func() { saveConcurrentReport(t, report) })
	l := matrixPrepare(t, hops, addresses...)
	report.NodeAddresses = append([]string(nil), l.nodeIPs...)
	var err error
	report.Assignments = l.runtimes
	report.Runtimes, err = evidence.Capture(context.Background(), l.runtimes.Inputs)
	require(t, err)
	l.guardIPv6Listeners(t)
	l.startMatrixServers(t)
	names := dag.Keys(l.topology.Routes)
	if len(names) == 0 {
		t.Fatal("empty protocol coverage")
	}
	report.Services, report.Profiles = len(l.topology.Services), names
	for _, name := range names {
		report.Paths[name] = l.topology.Routes[name].Path
	}
	endpoints, stops := concurrentClients(t, l, names, "concurrent")
	concurrentPhase(t, endpoints, "all-running", &report)
	stopped := names[len(names)/2]
	report.StoppedClient = stopped
	stops[stopped]()
	remaining := withoutEndpoint(endpoints, stopped)
	concurrentPhase(t, remaining, "one-client-stopped", &report)
	config, profile := engines.Client(l.topology, stopped, l.secrets)
	l.startClient(t, "restarted-"+stopped, profile.Engine, config)
	require(t, waitPort(profile.SOCKSPort))
	concurrentPhase(t, endpoints, "all-restored", &report)
}

func concurrentClients(t *testing.T, l *lab, names []string, label string) ([]traffic.Endpoint, map[string]func()) {
	t.Helper()
	var endpoints []traffic.Endpoint
	stops := map[string]func(){}
	for _, name := range names {
		config, profile := engines.Client(l.topology, name, l.secrets)
		stops[name] = l.startClient(t, label+"-"+name, profile.Engine, config)
		require(t, waitPort(profile.SOCKSPort))
		endpoints = append(endpoints, traffic.Endpoint{ID: name, Port: profile.SOCKSPort})
	}
	return endpoints, stops
}

func withoutEndpoint(endpoints []traffic.Endpoint, id string) []traffic.Endpoint {
	var remaining []traffic.Endpoint
	for _, endpoint := range endpoints {
		if endpoint.ID != id {
			remaining = append(remaining, endpoint)
		}
	}
	return remaining
}

func concurrentPhase(t *testing.T, endpoints []traffic.Endpoint, name string, suite *concurrentSuite) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	report, err := traffic.Run(ctx, endpoints, 25, 200*time.Millisecond)
	report.Phase = name
	suite.Phases = append(suite.Phases, report)
	if err != nil {
		t.Fatalf("%s: %v; missing origin arrivals: %v", name, err, report.Missing)
	}
	if len(report.Routes) != len(endpoints) || report.ConcurrentStreams != 2*len(endpoints) {
		t.Fatal("incomplete concurrent TCP/UDP coverage")
	}
	for id, result := range report.Routes {
		if result.TCPRounds != 25 || result.UDPRounds != 25 {
			t.Fatalf("%s did not finish every TCP/UDP round: %+v", id, result)
		}
	}
	t.Logf("%s: %d profiles, %d concurrent TCP/UDP streams, %d/25 origin barriers, %d ms",
		name, report.Profiles, report.ConcurrentStreams, report.BarrierRounds, report.DurationMS)
}

func installedTraffic(t *testing.T, l *lab, name string, suite *installedSuite) {
	t.Helper()
	endpoints, stops := concurrentClients(t, l, suite.Profiles, name)
	defer func() {
		for _, stop := range stops {
			stop()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	phase, err := traffic.Run(ctx, endpoints, 25, 200*time.Millisecond)
	phase.Phase = name
	suite.Phases = append(suite.Phases, phase)
	if err != nil {
		t.Fatalf("%s: %v; missing origin arrivals: %v", name, err, phase.Missing)
	}
	if len(phase.Routes) != len(suite.Profiles) || phase.BarrierRounds != 25 {
		t.Fatal("incomplete installed service traffic")
	}
	t.Logf("%s: %d profiles, %d TCP/UDP streams, %d barriers", name, phase.Profiles, phase.ConcurrentStreams, phase.BarrierRounds)
}

func saveConcurrentReport(t *testing.T, report concurrentSuite) {
	t.Helper()
	if !t.Skipped() {
		if err := evidence.Verify(report.Runtimes); err != nil {
			t.Error(err)
		}
	}
	directory := os.Getenv("NHOP_CONCURRENT_REPORT")
	if directory == "" {
		return
	}
	if !filepath.IsAbs(directory) {
		t.Error("NHOP_CONCURRENT_REPORT must be absolute; use just payload-concurrent")
		return
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Error(err)
		return
	}
	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	report.Status = "passed"
	if t.Failed() {
		report.Status = "failed"
	} else if t.Skipped() {
		report.Status = "skipped"
	}
	path := filepath.Join(directory, fmt.Sprintf("%d-hop.json", report.Hops))
	if err := safeio.JSON(path, report); err != nil {
		t.Error(err)
	}
}

func TestTrojanWebSocketClientRestarts(t *testing.T) {
	l := matrixLab(t, 1)
	for iteration := range 50 {
		t.Run(fmt.Sprintf("client-%02d", iteration), func(t *testing.T) {
			endpoints, stops := concurrentClients(t, l, []string{"trojan-ws"}, fmt.Sprintf("restart-%02d", iteration))
			defer stops["trojan-ws"]()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			phase, err := traffic.Run(ctx, endpoints, 3, 0)
			if err != nil {
				t.Fatalf("fresh client: %v; missing origin arrivals: %v", err, phase.Missing)
			}
			if phase.BarrierRounds != 3 {
				t.Fatal("fresh client did not complete every TCP/UDP barrier")
			}
		})
	}
}

func TestAllProtocolClientRestarts(t *testing.T) {
	l := matrixLab(t, 1)
	names := dag.Keys(l.topology.Routes)
	t.Logf("restart coverage: %v", names)
	for batch := range 5 {
		if !t.Run(fmt.Sprintf("batch-%02d", batch), func(t *testing.T) {
			// matrixStart binds process cleanup to this subtest, so every client
			// is stopped before the next batch reuses its listening ports.
			endpoints, _ := concurrentClients(t, l, names, fmt.Sprintf("batch-%02d", batch))
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			phase, err := traffic.Run(ctx, endpoints, 3, 0)
			if err != nil {
				t.Fatalf("fresh clients: %v; missing origin arrivals: %v", err, phase.Missing)
			}
			if phase.BarrierRounds != 3 || phase.ConcurrentStreams != 2*len(names) {
				t.Fatal("fresh clients did not complete every TCP/UDP barrier")
			}
			t.Logf("%d profiles, %d streams, %d barriers", phase.Profiles, phase.ConcurrentStreams, phase.BarrierRounds)
		}) {
			return
		}
	}
}

func TestPlannedListenersSurviveEphemeralExhaustion(t *testing.T) {
	if os.Getenv("NHOP_SOCKS_EXCLUSIVE") != "private-netns" {
		t.Skip("ephemeral exhaustion needs an isolated namespace; use just lab-socks-ports")
	}
	require(t, evidence.PrivatePortNamespace())
	l := &lab{}
	future := make([]int, 32)
	for index := range future {
		future[index] = l.port(t)
	}
	old, err := net.Listen("tcp4", "127.0.0.1:0")
	require(t, err)
	legacyAddress := old.Addr().String()
	require(t, old.Close())
	holdListenerPairs(t, make([]int, 32))
	occupied, err := net.Listen("tcp4", legacyAddress)
	if occupied != nil {
		occupied.Close()
	}
	if !errors.Is(err, syscall.EADDRINUSE) {
		t.Fatalf("automatic-port control did not reproduce the late bind conflict: %v", err)
	}
	holdListenerPairs(t, future)
	t.Log("32 planned TCP/UDP listeners started after all 32 automatic TCP/UDP ports were occupied")
}
