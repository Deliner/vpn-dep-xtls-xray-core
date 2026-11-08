package acceptance

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/engines"
)

type protocolFixture struct {
	topology  *model.Topology
	secrets   model.Secrets
	directory string
}

func protocolLab(t *testing.T, hops int) *protocolFixture {
	t.Helper()
	fixture := &protocolFixture{directory: t.TempDir(), secrets: model.Secrets{Services: map[string]model.Secret{}},
		topology: &model.Topology{Definition: model.Definition{Name: "tun-matrix", Nodes: map[string]*model.Node{}, Routes: map[string]*model.Route{}},
			Services: map[string]model.Endpoint{}, Successors: map[string]string{}}}
	var specs []protocol.Spec
	for _, spec := range protocol.All() {
		if spec.Status == "implemented" {
			specs = append(specs, spec)
		}
	}
	if len(specs) != 23 {
		t.Fatal("review TUN acceptance coverage when the implemented catalog changes")
	}
	target, trust := protocolDonor(t)
	for hop := range hops {
		fixture.node(t, hop, specs, target, trust)
	}
	for index, spec := range specs {
		route := &model.Route{Client: model.Client{SOCKSPort: 12080}}
		for hop := range hops {
			route.Path = append(route.Path, fmt.Sprintf("node%d.%s", hop, specs[(index+hop)%len(specs)].ID))
		}
		fixture.topology.Routes[spec.ID] = route
		for hop, name := range route.Path {
			if hop+1 < len(route.Path) {
				fixture.topology.Successors[name] = route.Path[hop+1]
			}
		}
	}
	fixture.start(t)
	return fixture
}

func (f *protocolFixture) node(t *testing.T, hop int, specs []protocol.Spec, target, trust string) {
	t.Helper()
	id := fmt.Sprintf("node%d", hop)
	node := &model.Node{Address: fmt.Sprintf("198.51.100.%d", 7+hop*2), Firewall: "preserve",
		BridgePortStart: 30000 + hop*1000, Services: map[string]*model.Service{}}
	f.topology.Nodes[id] = node
	for index, spec := range specs {
		service := model.Service{Protocol: spec.ID, Listen: node.Address, Port: 24443 + hop*100 + index, Path: "/vpn"}
		if spec.TLS {
			service.ServerName = "vpn.example.invalid"
		}
		if spec.Donor {
			service.ServerName, service.Target = "donor.example.invalid", target
		}
		secret, err := credentials.NewSecret(spec.ID)
		if err != nil {
			t.Fatal(err)
		}
		if err := credentials.EnsureTLS(service, &secret); err != nil {
			t.Fatal(err)
		}
		if spec.ID == "shadowtls-v3" {
			secret.TrustCertificate = trust
		}
		name := id + "." + spec.ID
		node.Services[spec.ID] = &service
		f.topology.Services[name] = model.Endpoint{Node: id, ID: spec.ID, Service: service}
		f.secrets.Services[name] = secret
	}
}

func protocolDonor(t *testing.T) (string, string) {
	t.Helper()
	secret := model.Secret{}
	if err := credentials.EnsureTLS(model.Service{Protocol: "trojan", ServerName: "donor.example.invalid"}, &secret); err != nil {
		t.Fatal(err)
	}
	certificate, err := tls.X509KeyPair([]byte(secret.Certificate), []byte(secret.CertificateKey))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("cover")) }))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, CurvePreferences: []tls.CurveID{tls.X25519}, Certificates: []tls.Certificate{certificate}}
	server.EnableHTTP2 = true
	server.Config.ErrorLog = log.New(io.Discard, "", 0)
	server.StartTLS()
	t.Cleanup(server.Close)
	return server.Listener.Addr().String(), secret.Certificate
}

func (f *protocolFixture) start(t *testing.T) {
	t.Helper()
	compiler := engines.NewCompiler(f.topology)
	for _, id := range dag.Keys(f.topology.Nodes) {
		data, err := compiler.Node(id, f.secrets)
		if err != nil {
			t.Fatal(err)
		}
		for _, program := range engines.Programs(data) {
			config := engines.Object(program.Config)
			for _, raw := range config["outbounds"].([]any) {
				out := raw.(map[string]any)
				if out["protocol"] == "freedom" {
					settings := out["settings"].(map[string]any)
					rules, _ := settings["finalRules"].([]any)
					settings["finalRules"] = append(rules, model.Object{"action": "allow", "network": "tcp,udp",
						"ip": []string{"198.18.0.2/32", "2001:db8:feed::2/128"}, "port": "443,18080,18081"})
				}
			}
			protocolServer(t, f.directory, id+"-"+program.Name, program.Name, config)
		}
	}
}

func protocolServer(t *testing.T, directory, label, engine string, config model.Object) {
	t.Helper()
	path := filepath.Join(directory, label+".json")
	if err := safeio.JSON(path, config); err != nil {
		t.Fatal(err)
	}
	binary := artifact(t, engine).Path
	result, err := process.Run(context.Background(), 10*time.Second, append([]string{binary}, engines.Arguments(engine, path, true)...)...)
	if err != nil || !result.Success {
		t.Fatalf("protocol server %s rejected: %v %s", label, err, result.Output)
	}
	output, err := os.Create(filepath.Join(directory, label+".log"))
	if err != nil {
		t.Fatal(err)
	}
	command := process.Command(context.Background(), binary, engines.Arguments(engine, path, false)...)
	command.Stdout, command.Stderr = output, output
	if err := command.Start(); err != nil {
		output.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		process.Stop(command)
		command.Wait()
		output.Close()
		data, err := os.ReadFile(output.Name())
		if err != nil {
			t.Error(err)
		}
		t.Logf("closed protocol server %s log: %s", label, data)
	})
}

func (f *protocolFixture) request(t *testing.T, name string) session.Request {
	t.Helper()
	request := runtimeRequest(t)
	directory := filepath.Dir(request.Directory)
	config, metadata := engines.Client(f.topology, name, f.secrets)
	var err error
	metadata.ConfigSHA256, err = safeio.Digest(config)
	if err != nil {
		t.Fatal(err)
	}
	catalog := model.Profiles{SchemaVersion: model.ClientProfilesVersion, TopologyID: "tun-matrix", Routes: map[string]model.Profile{name: metadata}}
	if err := os.Mkdir(filepath.Join(directory, "clients"), 0700); err != nil {
		t.Fatal(err)
	}
	for path, value := range map[string]any{"client-profiles.json": catalog, "clients/" + name + ".json": config} {
		if err := safeio.JSON(filepath.Join(directory, path), value); err != nil {
			t.Fatal(err)
		}
	}
	imported := filepath.Join(directory, "imported.json")
	data, err := cliCommand(t, "import", "--bundle", directory, "--route", name, "--output", imported).Output()
	if err != nil {
		t.Fatalf("public import: %v %s", err, data)
	}
	request.Profile, err = profile.Load(imported)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TUN route=%s engine=%s path=%v", name, metadata.Engine, metadata.Path)
	return request
}
