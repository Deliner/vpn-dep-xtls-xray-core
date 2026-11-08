package reboot

import (
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/engines"
)

func (f *fixture) materials(t *testing.T) {
	t.Helper()
	address := f.config["boot-exit"].PeerIP
	service := model.Service{Protocol: model.Shadowsocks, Listen: address, Port: 24443}
	node := &model.Node{Address: address, Firewall: "preserve", BridgePortStart: 30000, Services: map[string]*model.Service{"proxy": &service}}
	topology := &model.Topology{Definition: model.Definition{Name: "reboot", Nodes: map[string]*model.Node{"exit": node},
		Routes: map[string]*model.Route{"main": {Path: []string{"exit.proxy"}, Client: model.Client{SOCKSPort: 12080}}}},
		Services: map[string]model.Endpoint{"exit.proxy": {Node: "exit", ID: "proxy", Service: service}}, Successors: map[string]string{}}
	secret, err := credentials.NewSecret(model.Shadowsocks)
	require(t, err)
	secrets := model.Secrets{Services: map[string]model.Secret{"exit.proxy": secret}}
	data, err := engines.NewCompiler(topology).Node("exit", secrets)
	require(t, err)
	programs := engines.Programs(data)
	if len(programs) != 1 || programs[0].Name != "xray" {
		t.Fatal("unexpected reboot exit runtime")
	}
	server := engines.Object(programs[0].Config)
	allowOrigins(server, address)
	require(t, safeio.JSON(filepath.Join(f.root, "server.json"), server))
	config, metadata := engines.Client(topology, "main", secrets)
	metadata.ConfigSHA256, err = safeio.Digest(config)
	require(t, err)
	require(t, os.MkdirAll(filepath.Join(f.root, "bundle/clients"), 0700))
	require(t, safeio.JSON(filepath.Join(f.root, "bundle/clients/main.json"), config))
	require(t, safeio.JSON(filepath.Join(f.root, "bundle/client-profiles.json"), model.Profiles{SchemaVersion: model.ClientProfilesVersion, TopologyID: "reboot", Routes: map[string]model.Profile{"main": metadata}}))
	f.trust(t)
	f.inventory(t)
}

func allowOrigins(config model.Object, address string) {
	for _, raw := range config["outbounds"].([]any) {
		out := raw.(map[string]any)
		if out["protocol"] != "freedom" {
			continue
		}
		out["sendThrough"] = address
		settings := out["settings"].(map[string]any)
		rules, _ := settings["finalRules"].([]any)
		settings["finalRules"] = append(rules, model.Object{"action": "allow", "network": "tcp,udp", "ip": []string{address + "/32", contract.OriginIP + "/32"}, "port": "443,18080,18081"})
	}
}

func (f *fixture) trust(t *testing.T) {
	t.Helper()
	secret := model.Secret{}
	require(t, credentials.EnsureTLS(model.Service{Protocol: "trojan", ServerName: "dns.example.invalid"}, &secret))
	require(t, safeio.Create(filepath.Join(f.root, "dns.crt"), []byte(secret.Certificate), 0600))
	require(t, safeio.Create(filepath.Join(f.root, "dns.key"), []byte(secret.CertificateKey), 0600))
}

func (f *fixture) inventory(t *testing.T) {
	t.Helper()
	artifacts := map[string]session.Artifact{}
	for _, name := range []string{"xray", "sing-box", "libcronet.so"} {
		digest, err := safeio.FileHash(filepath.Join(f.root, "bin", name))
		require(t, err)
		artifacts[name] = session.Artifact{Path: contract.Root + "/bin/" + name, SHA256: digest}
	}
	value := controller.Runtimes{SchemaVersion: 1, Xray: artifacts["xray"], SingBox: artifacts["sing-box"], Cronet: artifacts["libcronet.so"]}
	require(t, safeio.JSON(filepath.Join(f.root, "runtimes.json"), value))
}
