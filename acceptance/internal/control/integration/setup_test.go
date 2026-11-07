package integration

import (
	"context"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"vpn-script/internal/control/topology"
	"vpn-script/internal/core/dag"
	"vpn-script/internal/core/model"
	"vpn-script/internal/core/protocol"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/engines"
	"vpn-script/internal/runtime/xray"
)

func setup(t *testing.T) *lab {
	t.Helper()
	binary := os.Getenv("XRAY_BINARY")
	if binary == "" {
		t.Skip("set XRAY_BINARY to run real payload tests")
	}
	version, err := process.Run(context.Background(), 10*time.Second, binary, "version")
	require(t, err)
	if !strings.HasPrefix(string(version.Output), "Xray "+xray.Version+" ") {
		t.Fatal("use the pinned Xray version")
	}
	top, err := topology.Load("../../../examples/nhop/topology.yaml")
	require(t, err)
	l := &lab{directory: t.TempDir(), topology: top,
		secrets: model.Secrets{Services: map[string]model.Secret{}}, servers: map[string]func(){}, clients: map[string]int{}}
	l.runtimes = selectRuntimes(t, dag.Keys(top.Nodes), []string{"xray"})
	l.origin = startOrigins(t)
	l.configure(t, donor(t))
	l.startServers(t)
	l.startClients(t)
	return l
}

func (l *lab) configure(t *testing.T, target string) {
	t.Helper()
	// Test-only mapping of three distinct machines to three loopback processes.
	for _, node := range l.topology.Nodes {
		node.Address = "127.0.0.1"
	}
	for name, endpoint := range l.topology.Services {
		endpoint.Service.Listen, endpoint.Service.Port = "127.0.0.1", l.port(t)
		if endpoint.Service.Protocol == model.Reality {
			endpoint.Service.ServerName, endpoint.Service.Target = "donor.example.invalid", target
		}
		l.topology.Services[name] = endpoint
		secret, err := xray.NewSecret(endpoint.Service.Protocol)
		require(t, err)
		l.secrets.Services[name] = secret
	}
}

func (l *lab) startServers(t *testing.T) {
	t.Helper()
	compiler := engines.NewCompiler(l.topology)
	for _, id := range l.topology.Order {
		data, err := compiler.Node(id, l.secrets)
		require(t, err)
		// Allow only the lab origins after the production egress rules.
		for _, out := range data.Xray.Outbounds {
			if out["protocol"] == "freedom" {
				localFreedom(out, "",
					model.Object{"action": "allow", "network": "tcp", "ip": []string{"127.0.0.1/32"}, "port": strconv.Itoa(l.origin.http.Listener.Addr().(*net.TCPAddr).Port)},
					model.Object{"action": "allow", "network": "udp", "ip": []string{"127.0.0.1/32"}, "port": strconv.Itoa(l.origin.udp.LocalAddr().(*net.UDPAddr).Port)})
			}
		}
		l.servers[id] = l.start(t, id, l.serverBinary(id, "xray"), data.Xray)
	}
}

func (l *lab) startClients(t *testing.T) {
	t.Helper()
	for id, route := range l.topology.Routes {
		first, port := route.Path[0], l.port(t)
		config := xray.Client(xray.Outbound(l.topology, first, l.secrets.Services[first]), port)
		l.start(t, "client-"+id, l.clientBinary("xray"), config)
		l.clients[id] = port
	}
}

func matrixLab(t *testing.T, hops int) *lab {
	t.Helper()
	l := matrixPrepare(t, hops)
	l.startMatrixServers(t)
	return l
}

func (l *lab) startMatrixServers(t *testing.T) {
	t.Helper()
	compiler := engines.NewCompiler(l.topology)
	for _, id := range dag.Keys(l.topology.Nodes) {
		l.matrixServer(t, id, compiler, "")
	}
}

func matrixPrepare(t *testing.T, hops int, addresses ...string) *lab {
	t.Helper()
	if os.Getenv("XRAY_BINARY") == "" || os.Getenv("SINGBOX_BINARY") == "" {
		t.Skip("requires pinned Xray and sing-box")
	}
	l := &lab{directory: t.TempDir(), nodeIPs: matrixAddresses(t, hops, addresses), secrets: model.Secrets{Services: map[string]model.Secret{}}, servers: map[string]func(){}, clients: map[string]int{}}
	l.origin = startOrigins(t)
	l.topology = &model.Topology{Definition: model.Definition{Name: "matrix", Nodes: map[string]*model.Node{}, Routes: map[string]*model.Route{}}, Services: map[string]model.Endpoint{}, Successors: map[string]string{}}
	certificate := donorCertificate(t)
	target := startDonor(t, certificate)
	trust := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}))
	l.origin.http.URL = strings.Replace(l.origin.http.URL, "127.0.0.1", "198.18.0.2", 1)
	var specs []protocol.Spec
	for _, spec := range protocol.All() {
		if spec.Status == "implemented" {
			specs = append(specs, spec)
		}
	}
	for hop := range hops {
		l.matrixNode(t, hop, specs, target, trust)
	}
	for index, spec := range specs {
		route := &model.Route{Client: model.Client{SOCKSPort: l.port(t)}}
		for hop := range hops {
			route.Path = append(route.Path, fmt.Sprintf("node%d.%s", hop, specs[(index+hop)%len(specs)].ID))
		}
		l.topology.Routes[spec.ID] = route
		for i, name := range route.Path {
			next := ""
			if i+1 < len(route.Path) {
				next = route.Path[i+1]
			}
			l.topology.Successors[name] = next
		}
	}
	l.runtimes = selectRuntimes(t, dag.Keys(l.topology.Nodes), []string{"xray", "sing-box"})
	return l
}

func (l *lab) matrixNode(t *testing.T, index int, specs []protocol.Spec, target, trust string) {
	t.Helper()
	id := fmt.Sprintf("node%d", index)
	node := &model.Node{Address: l.nodeIPs[index], Firewall: "preserve", BridgePortStart: 20000 + index*1000, Services: map[string]*model.Service{}}
	l.topology.Nodes[id] = node
	shared := l.port(t)
	t.Logf("%s: VLESS/TCP and TUIC/UDP share %s", id, net.JoinHostPort(node.Address, strconv.Itoa(shared)))
	for _, spec := range specs {
		service := model.Service{Protocol: spec.ID, Listen: node.Address, Port: l.matrixPort(t, spec.ID, shared), Path: "/vpn"}
		if spec.TLS {
			service.ServerName = "vpn.example.invalid"
		}
		if spec.Donor {
			service.ServerName, service.Target = "donor.example.invalid", target
		}
		name := id + "." + spec.ID
		node.Services[spec.ID] = &service
		l.topology.Services[name] = model.Endpoint{Node: id, ID: spec.ID, Service: service}
		secret, err := credentials.NewSecret(spec.ID)
		require(t, err)
		require(t, credentials.EnsureTLS(service, &secret))
		if spec.ID == "shadowtls-v3" {
			secret.TrustCertificate = trust
		}
		l.secrets.Services[name] = secret
	}
	if node.Services[model.Reality].Port != shared || node.Services["tuic"].Port != shared {
		t.Fatal("protocol matrix lost its shared TCP/UDP listener pair")
	}
}

func (l *lab) matrixPort(t *testing.T, protocol string, shared int) int {
	t.Helper()
	if protocol == model.Reality || protocol == "tuic" {
		return shared
	}
	return l.port(t)
}

func (l *lab) matrixServer(t *testing.T, id string, compiler *engines.Compiler, fault string) {
	t.Helper()
	data, err := compiler.Node(id, l.secrets)
	require(t, err)
	stops := []func(){}
	for _, program := range engines.Programs(data) {
		program = localProgram(program)
		label := id + "-" + program.Name
		if fault == "missing-routes" {
			dropInboundRoutes(program.Name, program.Config)
		}
		if fault == "isolated-routing" {
			useIsolatedOrigin(program.Name, program.Config)
		}
		label += fault
		stops = append(stops, matrixStart(t, l.directory, label, program.Name, l.serverBinary(id, program.Name), program.Config))
	}
	l.servers[id] = func() {
		for _, stop := range stops {
			stop()
		}
	}
}

func localProgram(program model.Program) model.Program {
	config := engines.Object(program.Config)
	bindLoopback(program.Name, config)
	labLogging(program.Name, config)
	for _, raw := range config["outbounds"].([]any) {
		out := raw.(map[string]any)
		if out["protocol"] == "freedom" {
			localFreedom(out, "127.0.0.1:0", model.Object{"action": "allow", "ip": []string{"127.0.0.1/32"}, "network": "tcp,udp"})
		}
	}
	if program.Name == "sing-box" {
		for _, raw := range config["route"].(map[string]any)["rules"].([]any) {
			rule := raw.(map[string]any)
			if rule["outbound"] == "exit" {
				rule["override_address"] = "127.0.0.1"
			}
		}
	}
	program.Config = config
	return program
}

func udpPort(t *testing.T) int {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require(t, err)
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).Port
}

func installedPublicPorts(t *testing.T, l *lab) {
	t.Helper()
	for protocol, port := range map[string]int{model.Reality: 443, "tuic": 443, "hysteria2": 22} {
		name := "node0." + protocol
		endpoint := l.topology.Services[name]
		endpoint.Service.Port = port
		l.topology.Services[name] = endpoint
		l.topology.Nodes["node0"].Services[protocol].Port = port
	}
	checkTCPListener(t, "127.0.0.1:22")
}

func checkTCPListener(t *testing.T, address string) {
	t.Helper()
	connection, err := net.DialTimeout("tcp4", address, time.Second)
	require(t, err)
	require(t, connection.Close())
}

func installedWildcards(data *model.NodeConfig) {
	tags := []string{"node0.trojan", "node0.hysteria2-gecko"}
	for _, program := range data.Programs {
		for _, raw := range program.Config["inbounds"].([]any) {
			inbound := raw.(map[string]any)
			if slices.Contains(tags, fmt.Sprint(inbound["tag"])) {
				inbound["listen"] = "0.0.0.0"
			}
		}
	}
	for index, listener := range data.Listeners {
		if slices.Contains(tags, listener.Tag) {
			data.Listeners[index].Address = "0.0.0.0"
		}
	}
}

func installedWildcardTags(t *testing.T, data model.NodeConfig) []string {
	t.Helper()
	var tags []string
	for _, listener := range data.Listeners {
		if listener.Address == "0.0.0.0" {
			tags = append(tags, listener.Tag)
		}
	}
	if len(tags) != 2 {
		t.Fatal("installed fixture lost its two public wildcard listeners")
	}
	return tags
}
