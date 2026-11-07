package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"vpn-script/internal/control/client"
	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/control/integration/evidence/diagnostics"
	"vpn-script/internal/control/integration/selection"
	"vpn-script/internal/control/integration/traffic"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/engines"
	"vpn-script/internal/runtime/xray"
)

func (l *lab) start(t *testing.T, label, binary string, config model.Xray) func() {
	t.Helper()
	stop := matrixStart(t, l.directory, label, "xray", binary, engines.Object(config))
	port, err := strconv.Atoi(fmt.Sprint(config.Inbounds[0]["port"]))
	require(t, err)
	require(t, waitPort(port))
	return stop
}

func waitPort(port int) error {
	for range 100 {
		connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 100*time.Millisecond)
		if err == nil {
			connection.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("Xray listener did not start")
}

func (l *lab) http(port int) ([]byte, error) {
	result, err := process.Run(context.Background(), 6*time.Second, "curl", "--disable", "--silent", "--show-error", "--fail", "--max-time", "4",
		"--noproxy", "", "--proxy", "socks5h://127.0.0.1:"+strconv.Itoa(port), l.origin.http.URL)
	return result.Output, err
}

func matrixStart(t *testing.T, directory, label, engine, binary string, config model.Object) func() {
	t.Helper()
	require(t, client.ValidateBinary(context.Background(), engine, binary, process.Run))
	path := filepath.Join(directory, label+".json")
	bindLoopback(engine, config)
	labLogging(engine, config)
	require(t, safeio.JSON(path, config))
	result, err := process.Run(context.Background(), 10*time.Second, append([]string{binary}, engines.Arguments(engine, path, true)...)...)
	if err != nil || !result.Success {
		t.Fatalf("%s config rejected: %s", label, result.Output)
	}
	logPath := filepath.Join(directory, label+".log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	require(t, err)
	command := process.Command(context.Background(), binary, engines.Arguments(engine, path, false)...)
	command.Stdout, command.Stderr = log, log
	require(t, command.Start())
	var once sync.Once
	stop := func() { once.Do(func() { _ = process.Stop(command); _ = command.Wait(); _ = log.Close() }) }
	t.Cleanup(func() {
		stop()
		if err := diagnostics.RetainRuntimeLog(os.Getenv("NHOP_RUNTIME_LOG_DIR"), t.Name(), label, logPath); err != nil {
			t.Error(err)
		}
		if t.Failed() {
			raw, _ := os.ReadFile(logPath)
			t.Logf("%s: %s", label, raw)
		}
	})
	time.Sleep(200 * time.Millisecond)
	return stop
}

func selectRuntimes(t *testing.T, nodes, engineNames []string) selection.Set {
	t.Helper()
	options := selection.Options{Servers: map[string]evidence.Input{}, Clients: map[string]evidence.Input{}, Updated: map[string]evidence.Input{}}
	for _, engine := range engineNames {
		prefix := strings.ToUpper(strings.ReplaceAll(engine, "-", ""))
		input := evidence.Input{ExpectedVersion: engines.Runtime(engine).Version, Path: os.Getenv(prefix + "_BINARY")}
		options.Servers[engine] = input
		if path := os.Getenv(prefix + "_CLIENT_BINARY"); path != "" {
			input.Path = path
			options.Clients[engine] = input
		}
		if path := os.Getenv(prefix + "_UPDATED_BINARY"); path != "" {
			input.Path = path
			options.Updated[engine] = input
		}
	}
	if updated := os.Getenv("NHOP_UPDATED_NODES"); updated != "" {
		options.UpdatedNodes = strings.Split(updated, ",")
	}
	result, err := selection.New(nodes, options)
	require(t, err)
	return result
}

func (l *lab) clientBinary(engine string) string {
	return l.runtimes.Inputs[l.runtimes.Clients[engine]].Path
}

func (l *lab) serverBinary(node, engine string) string {
	return l.runtimes.Inputs[l.runtimes.Nodes[node][engine]].Path
}

func (l *lab) startClient(t *testing.T, label, engine string, config model.Object) func() {
	t.Helper()
	return matrixStart(t, l.directory, label, engine, l.clientBinary(engine), config)
}

func bindLoopback(engine string, config model.Object) {
	if engine != "xray" {
		return
	}
	// Bind each lab source to the destination's IP family. Darwin can otherwise
	// give a dual-stack UDP socket a port held by a single-family listener.
	// Origin overrides and private bridges remain IPv4; public hops may be IPv6.
	for _, raw := range config["outbounds"].([]any) {
		out := raw.(map[string]any)
		if out["protocol"] != "blackhole" {
			out["sendThrough"] = outboundLoopback(out)
		}
	}
}

func labLogging(engine string, config model.Object) {
	if os.Getenv("NHOP_PAYLOAD_DEBUG") != "1" {
		return
	}
	key := "level"
	if engine == "xray" {
		key = "loglevel"
	}
	config["log"] = model.Object{key: "debug"}
}

func dropInboundRoutes(engine string, config model.Object) {
	key, selector := "routing", "inboundTag"
	if engine == "sing-box" {
		key, selector = "route", "inbound"
	}
	routing := config[key].(map[string]any)
	var remaining []any
	for _, raw := range routing["rules"].([]any) {
		if _, matched := raw.(map[string]any)[selector]; !matched {
			remaining = append(remaining, raw)
		}
	}
	routing["rules"] = remaining
}

func captureLabRuntimes(t *testing.T, l *lab) {
	t.Helper()
	observed, err := evidence.Capture(context.Background(), l.runtimes.Inputs)
	require(t, err)
	t.Cleanup(func() { require(t, evidence.Verify(observed)) })
	for role, identity := range observed {
		t.Logf("%s SHA256=%s", role, identity.SHA256)
	}
}

func routingProbe(t *testing.T, l *lab, port int, denied bool) {
	t.Helper()
	httpHits, udpHits := l.origin.httpHits.Load(), l.origin.udpHits.Load()
	body, err := l.http(port)
	if denied && err == nil {
		t.Fatal("unmatched TCP route reached origin")
	}
	if !denied && (err != nil || !bytes.Equal(body, payload)) {
		t.Fatalf("positive TCP control failed: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	probe, err := traffic.OpenUDPProbe(ctx, port)
	require(t, err)
	defer probe.Close()
	got, err := probe.Exchange(l.origin.udp.LocalAddr().(*net.UDPAddr).Port, []byte("routing"), 2*time.Second)
	if denied && err == nil {
		t.Fatal("unmatched UDP route replied")
	}
	if !denied && (err != nil || !bytes.Equal(got, []byte("routing"))) {
		t.Fatalf("positive UDP control failed: %v", err)
	}
	if denied && (l.origin.httpHits.Load() != httpHits || l.origin.udpHits.Load() != udpHits) {
		t.Fatal("unmatched traffic reached origin even though its client reported failure")
	}
}

func useIsolatedOrigin(engine string, config model.Object) {
	if engine == "xray" {
		for _, raw := range config["outbounds"].([]any) {
			out := raw.(map[string]any)
			if out["protocol"] == "freedom" {
				localFreedom(out, "", model.Object{"action": "allow", "ip": []string{"198.18.0.2/32"}, "network": "tcp,udp"})
			}
		}
		return
	}
	for _, raw := range config["route"].(map[string]any)["rules"].([]any) {
		rule := raw.(map[string]any)
		delete(rule, "override_address")
	}
}

func localFreedom(out model.Object, redirect string, allow ...model.Object) {
	settings := engines.Object(out["settings"])
	if settings == nil {
		settings = model.Object{}
	}
	delete(settings, "redirect")
	if redirect != "" {
		settings["redirect"] = redirect
	}
	rules, _ := settings["finalRules"].([]any)
	for _, rule := range allow {
		rules = append(rules, rule)
	}
	settings["finalRules"] = rules
	out["settings"] = settings
}

func udpChecks(t *testing.T, probe *traffic.UDPProbe, target int, denied bool, address ...[4]byte) {
	t.Helper()
	rounds, timeout := 1, 2*time.Second
	if denied {
		rounds, timeout = 3, 300*time.Millisecond
	}
	for sequence := range rounds {
		data := []byte(fmt.Sprintf("routing-control-%d", sequence))
		got, err := probe.Exchange(target, data, timeout, address...)
		if denied {
			var timeout net.Error
			if !errors.As(err, &timeout) || !timeout.Timeout() {
				t.Fatalf("rejected UDP needs an intact association and no reply; got=%x, error=%v", got, err)
			}
		} else if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("positive UDP control failed: %v", err)
		}
	}
}

func probeUDPPolicy(t *testing.T, port, first, second int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	probe, err := traffic.OpenUDPProbe(ctx, port)
	require(t, err)
	defer probe.Close()
	address := [4]byte{198, 18, 0, 2}
	udpChecks(t, probe, first, false, address)
	udpChecks(t, probe, 443, true, address)
	udpChecks(t, probe, second, false, address)
	udpChecks(t, probe, first, false, address)
}

func TestSOCKSUDPAssociationPortsAreExclusive(t *testing.T) {
	if os.Getenv("NHOP_SOCKS_EXCLUSIVE") != "private-netns" {
		t.Skip("explicit isolated Linux namespace regression; use just lab-socks-ports")
	}
	require(t, evidence.PrivatePortNamespace())
	binary := os.Getenv("XRAY_BINARY")
	observed, err := evidence.Capture(context.Background(), map[string]evidence.Input{"xray": {Path: binary, ExpectedVersion: xray.Version}})
	require(t, err)
	t.Cleanup(func() { require(t, evidence.Verify(observed)) })
	config := xray.Client(model.Object{"tag": "closed", "protocol": "blackhole"}, 1080)
	stop := matrixStart(t, t.TempDir(), "exclusive-socks", "xray", binary, engines.Object(config))
	defer stop()
	require(t, waitPort(1080))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ports, err := traffic.AssociationPorts(ctx, 1080, 20000, 33)
	seen := map[int]bool{}
	for _, port := range ports {
		if seen[port] || port < 45000 || port > 45031 {
			t.Fatalf("UDP association reused or escaped its port: %d; allocations: %v", port, ports)
		}
		seen[port] = true
	}
	if len(ports) != 32 || !errors.Is(err, traffic.ErrUDPAllocation) {
		t.Fatalf("expected 32 exclusive listeners then allocation refusal; ports=%v, error=%v", ports, err)
	}
	t.Logf("32 unique UDP ports; association 33 refused; Xray SHA256=%s", observed["xray"].SHA256)
}
