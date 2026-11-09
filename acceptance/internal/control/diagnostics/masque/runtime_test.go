package masque

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/xray"
)

func setup(t *testing.T) *fixture {
	t.Helper()
	requireIsolation(t)
	directory, err := filepath.Abs(os.Getenv("NHOP_MASQUE_REPORT"))
	if err != nil {
		t.Fatal(err)
	}
	if err := safeio.CheckAccess(directory, os.ModeDir|0700, 0, 0); err != nil {
		t.Fatal(err)
	}
	// Registered before children: evidence reads only closed, complete process logs.
	t.Cleanup(func() { attributeQueueDrops(t, directory) })
	f := &fixture{directory: directory, binary: os.Getenv("XRAY_BINARY")}
	service := model.Service{Protocol: "masque-h3", Listen: "198.51.100.2", Port: 24443, ServerName: "masque.example.invalid"}
	secret, err := xray.NewSecret(service.Protocol)
	if err != nil {
		t.Fatal(err)
	}
	if err := credentials.EnsureTLS(service, &secret); err != nil {
		t.Fatal(err)
	}
	server := model.Xray{Log: model.Object{"loglevel": "info"}, Inbounds: []model.Object{xray.Inbound("masque", service, secret)},
		Outbounds: []model.Object{{"protocol": "freedom", "settings": model.Object{"finalRules": []model.Object{{
			"action": "allow", "network": "tcp,udp", "ip": []string{"198.18.0.2/32"}, "port": "18080,18081"}}}}}}
	writeRuntimeConfig(t, directory, "server", server)
	client := xray.Client(xray.Remote("exit", "198.51.100.2", service, secret), socksPort)
	client.Log = model.Object{"loglevel": "info"}
	writeRuntimeConfig(t, directory, "client", client)
	startOrigins(t, directory)
	f.restart(t, "server")
	f.restart(t, "client")
	return f
}

func writeRuntimeConfig(t *testing.T, directory, role string, config model.Xray) {
	t.Helper()
	if err := configureCongestion(config, os.Getenv("NHOP_MASQUE_CONGESTION")); err != nil {
		t.Fatal(err)
	}
	writeJSON(t, filepath.Join(directory, role+".json"), config)
}

func configureCongestion(config model.Xray, policy string) error {
	if policy == "" {
		return nil
	}
	if policy != "bbr" && policy != "reno" {
		return fmt.Errorf("unreviewed diagnostic congestion policy: %q", policy)
	}
	var target model.Object
	for _, nodes := range [][]model.Object{config.Inbounds, config.Outbounds} {
		for _, node := range nodes {
			if node["protocol"] != "masque" {
				continue
			}
			stream, ok := node["streamSettings"].(model.Object)
			if !ok || stream == nil || target != nil {
				return fmt.Errorf("ambiguous diagnostic MASQUE stream")
			}
			target = stream
		}
	}
	if target == nil {
		return fmt.Errorf("diagnostic MASQUE stream not found")
	}
	if _, exists := target["finalmask"]; exists {
		return fmt.Errorf("refusing to replace an existing diagnostic mask")
	}
	target["finalmask"] = model.Object{"quicParams": model.Object{"congestion": policy}}
	return nil
}

func TestCongestionSelectionPreservesDefaultAndUnrelatedConfig(t *testing.T) {
	for _, policy := range []string{"", "bbr", "reno"} {
		stream := model.Object{"network": "masque", "security": "tls"}
		config := model.Xray{Inbounds: []model.Object{{"protocol": "socks"}}, Outbounds: []model.Object{
			{"protocol": "blackhole"}, {"protocol": "masque", "streamSettings": stream}}}
		before, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		if err := configureCongestion(config, policy); err != nil {
			t.Fatal(err)
		}
		if policy != "" {
			mask, ok := stream["finalmask"].(model.Object)
			if !ok || mask["quicParams"].(model.Object)["congestion"] != policy {
				t.Fatal("selected policy did not reach the MASQUE stream")
			}
			delete(stream, "finalmask")
		}
		after, err := json.Marshal(config)
		if err != nil || string(before) != string(after) {
			t.Fatal("diagnostic selection changed unrelated configuration")
		}
	}
}

func TestCongestionSelectionRefusesUnsafeOverrides(t *testing.T) {
	for _, test := range []struct {
		policy string
		nodes  []model.Object
	}{
		{"unknown", nil}, {"reno", nil},
		{"reno", []model.Object{{"protocol": "masque"}}},
		{"reno", []model.Object{{"protocol": "masque", "streamSettings": model.Object{"finalmask": model.Object{}}}}},
		{"reno", []model.Object{{"protocol": "masque", "streamSettings": model.Object{}}, {"protocol": "masque", "streamSettings": model.Object{}}}},
	} {
		config := model.Xray{Outbounds: test.nodes}
		before, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		if err := configureCongestion(config, test.policy); err == nil {
			t.Fatal("unsafe congestion override was accepted")
		}
		after, err := json.Marshal(config)
		if err != nil || string(before) != string(after) {
			t.Fatal("refused diagnostic selection mutated configuration")
		}
	}
}

func (f *fixture) restart(t *testing.T, role string) {
	t.Helper()
	child, namespace := &f.client, ""
	if role == "server" {
		child, namespace = &f.server, exitNamespace
	}
	if *child != nil {
		(*child).stop(t)
	}
	f.sequence++
	*child = startManaged(t, f.directory, fmt.Sprintf("%s-%03d", role, f.sequence), namespace,
		f.binary, "run", "-c", filepath.Join(f.directory, role+".json"))
	if role == "client" {
		waitSOCKS(t, *child)
	}
}

func waitSOCKS(t *testing.T, child *managed) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", "127.0.0.1:10800", 100*time.Millisecond)
		if err == nil {
			connection.Close()
			return
		}
		select {
		case err := <-child.done:
			t.Fatalf("client exited before readiness: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatal("SOCKS listener did not become ready")
}
