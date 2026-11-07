package profile

import (
	"path/filepath"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/credentials"
	"vpn-script/internal/runtime/engines"
)

func exportFixture(t *testing.T, kind string) string {
	t.Helper()
	service := model.Service{Protocol: kind, Port: 24443, ServerName: "vpn.example.invalid", Target: "donor.example.invalid:443", Path: "/vpn"}
	top := &model.Topology{Definition: model.Definition{Name: "fixture", Nodes: map[string]*model.Node{"entry": {Address: "198.51.100.7"}},
		Routes: map[string]*model.Route{"main": {Path: []string{"entry.proxy"}, Client: model.Client{SOCKSPort: 12080}}}},
		Services: map[string]model.Endpoint{"entry.proxy": {Node: "entry", ID: "proxy", Service: service}}}
	secret, err := credentials.NewSecret(kind)
	if err != nil {
		t.Fatal(err)
	}
	if err := credentials.EnsureTLS(service, &secret); err != nil {
		t.Fatal(err)
	}
	config, metadata := engines.Client(top, "main", model.Secrets{Services: map[string]model.Secret{"entry.proxy": secret}})
	metadata.ConfigSHA256, err = safeio.Digest(config)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	if err := safeio.PrivateDir(filepath.Join(directory, "clients")); err != nil {
		t.Fatal(err)
	}
	files := map[string]any{"clients/main.json": config, "client-profiles.json": model.Profiles{SchemaVersion: model.ClientProfilesVersion, TopologyID: "fixture", Routes: map[string]model.Profile{"main": metadata}}}
	for path, value := range files {
		if err := safeio.JSON(filepath.Join(directory, path), value); err != nil {
			t.Fatal(err)
		}
	}
	return directory
}
