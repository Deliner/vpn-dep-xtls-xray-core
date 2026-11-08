package acceptance

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

func (f *fixture) traffic(t *testing.T, phase string) {
	t.Helper()
	var endpoints []traffic.Endpoint
	for _, name := range []string{"shadowsocks", "tuic"} {
		port := f.startClient(t, name, phase)
		endpoints = append(endpoints, traffic.Endpoint{ID: name, Port: port})
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	report, err := traffic.Run(ctx, endpoints, 3, 20*time.Millisecond)
	require(t, err)
	report.Phase = phase
	f.report.Traffic[phase] = report
	if report.BarrierRounds != 3 || report.ConcurrentStreams != 4 {
		t.Fatal("both engines did not cross every TCP/UDP barrier together")
	}
}

func (f *fixture) startClient(t *testing.T, name, phase string) int {
	t.Helper()
	f.topology.Routes[name].Client.SOCKSPort = freePort(t)
	config, profile := engines.Client(f.topology, name, f.secrets)
	path := filepath.Join(f.root, "reports", phase+"-"+name+".json")
	require(t, safeio.NewJSON(path, config))
	log, err := os.OpenFile(filepath.Join(f.root, "reports", phase+"-"+name+".log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	require(t, err)
	binary := filepath.Join(nodeBase, "current", profile.Engine)
	child := process.Command(context.Background(), binary, engines.Arguments(profile.Engine, path, false)...)
	child.Stdout, child.Stderr = log, log
	require(t, child.Start())
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	t.Cleanup(func() {
		process.Stop(child)
		<-done
		require(t, log.Close())
	})
	eventually(t, name+" client listener", 5*time.Second, func() bool {
		connection, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(profile.SOCKSPort)), 200*time.Millisecond)
		if err != nil {
			return false
		}
		connection.Close()
		return true
	})
	return profile.SOCKSPort
}
