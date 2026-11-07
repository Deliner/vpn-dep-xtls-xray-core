package bundle_test

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"vpn-script/internal/control/bundle"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
)

func TestStableSecretsAndPrivateFiles(t *testing.T) {
	top, output, installers := fixture(t)
	first, err := os.ReadFile(filepath.Join(output, "state.json"))
	must(t, err)
	must(t, bundle.Render(top, output, installers))
	second, err := os.ReadFile(filepath.Join(output, "state.json"))
	must(t, err)
	if string(first) != string(second) {
		t.Fatal("render rotated credentials")
	}
	must(t, filepath.WalkDir(output, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err == nil && info.Mode().Perm()&0077 != 0 {
			t.Errorf("private file permissions: %s", path)
		}
		return err
	}))
	_, err = bundle.Load(output)
	must(t, err)
}

func TestNodeSecretDistributionAndFailClosed(t *testing.T) {
	top, output, _ := fixture(t)
	var state model.Secrets
	must(t, safeio.LoadJSON(filepath.Join(output, "state.json"), &state))
	for id := range top.Nodes {
		path := filepath.Join(output, "nodes", id+".json")
		raw, err := os.ReadFile(path)
		must(t, err)
		for name, secret := range state.Services {
			if secret.PrivateKey != "" && strings.Contains(string(raw), secret.PrivateKey) != (top.Services[name].Node == id) {
				t.Fatal("private key distributed to an unrelated node")
			}
		}
		config, err := manifest.Load(path)
		must(t, err)
		for _, outbound := range config.Xray.Outbounds {
			if id != "exit" && outbound["protocol"] == "freedom" {
				t.Fatal("relay has a direct fallback")
			}
		}
	}
}

func TestCustomPortsAndSIP002(t *testing.T) {
	top, output, installers := fixture(t)
	var before model.Secrets
	must(t, safeio.LoadJSON(filepath.Join(output, "state.json"), &before))
	top.Nodes["entry"].SSH.Port = 22022
	endpoint := top.Services["entry.reality"]
	endpoint.Service.Port = 24443
	top.Services["entry.reality"] = endpoint
	top.Routes["main"].Client.SOCKSPort = 12080
	must(t, bundle.Render(top, output, installers))
	var after model.Secrets
	must(t, safeio.LoadJSON(filepath.Join(output, "state.json"), &after))
	if !reflect.DeepEqual(before, after) {
		t.Fatal("changing ports rotated credentials")
	}
	var profiles model.Profiles
	must(t, safeio.LoadJSON(filepath.Join(output, "client-profiles.json"), &profiles))
	link, err := url.Parse(profiles.Routes["main"].URI)
	must(t, err)
	if link.Port() != "24443" || profiles.Routes["main"].SOCKSPort != 12080 {
		t.Fatal("custom ports not propagated")
	}
	link, err = url.Parse(profiles.Routes["alternative"].URI)
	must(t, err)
	password, _ := link.User.Password()
	if link.User.Username() != model.SSMethod || password != after.Services["entry.shadowsocks"].Password {
		t.Fatal("AEAD-2022 URI is not SIP002 compliant")
	}
}

func TestTamperedBundleRejected(t *testing.T) {
	_, output, _ := fixture(t)
	path := filepath.Join(output, "nodes", "entry.json")
	raw, err := os.ReadFile(path)
	must(t, err)
	must(t, os.WriteFile(path, append(raw, ' '), 0600))
	if _, err := bundle.Load(output); err == nil {
		t.Fatal("tampered bundle accepted")
	}
	var config model.NodeConfig
	must(t, json.Unmarshal(raw, &config))
	config.Xray.Outbounds[0] = model.Object{"tag": "direct", "protocol": "freedom"}
	if err := manifest.Validate(config); err == nil {
		t.Fatal("tampered node generation accepted")
	}
}

func TestDeploymentDependenciesDeduplicateServiceRoutes(t *testing.T) {
	_, output, _ := fixture(t)
	deployment, err := bundle.Load(output)
	must(t, err)
	dependencies, err := bundle.Dependencies(output, deployment)
	must(t, err)
	expected := map[string][]string{"entry": {"relay"}, "relay": {"exit"}, "exit": {}}
	if !reflect.DeepEqual(dependencies, expected) {
		t.Fatalf("incorrect machine dependencies: %v", dependencies)
	}
}

func TestDeploymentRejectsInvalidProfileIdentity(t *testing.T) {
	for _, mutate := range []func(*model.Profiles){
		func(profiles *model.Profiles) { profiles.SchemaVersion = 3 },
		func(profiles *model.Profiles) { profiles.TopologyID = "wrong" },
		func(profiles *model.Profiles) { profiles.Routes = nil },
		func(profiles *model.Profiles) { profiles.Routes["bad/name"] = profiles.Routes["main"] },
	} {
		_, output, _ := fixture(t)
		deployment, err := bundle.Load(output)
		must(t, err)
		path := filepath.Join(output, "client-profiles.json")
		var profiles model.Profiles
		must(t, safeio.LoadJSON(path, &profiles))
		mutate(&profiles)
		must(t, safeio.JSON(path, profiles))
		deployment.ProfilesSHA256, err = safeio.FileHash(path)
		must(t, err)
		if _, err := bundle.Dependencies(output, deployment); err == nil {
			t.Fatal("invalid profile identity accepted")
		}
	}
}

func TestDeploymentRejectsInconsistentRoutePaths(t *testing.T) {
	valid := []string{"entry.reality", "relay.shadowsocks", "exit.reality"}
	cases := []struct {
		name        string
		path        []string
		hops        int
		entry, exit string
	}{
		{"empty", nil, 0, "entry", "exit"},
		{"hop-count", valid, 2, "entry", "exit"},
		{"entry", valid, 3, "relay", "exit"},
		{"exit", valid, 3, "entry", "relay"},
		{"unknown", []string{"entry.reality", "unknown.vpn", "exit.reality"}, 3, "entry", "exit"},
		{"self-edge", []string{"entry.reality", "entry.shadowsocks", "exit.reality"}, 3, "entry", "exit"},
		{"order", []string{"entry.reality", "exit.reality", "relay.reality"}, 3, "entry", "relay"},
		{"service", []string{"entry.bad.service", "relay.shadowsocks", "exit.reality"}, 3, "entry", "exit"},
		{"coverage", []string{"entry.reality"}, 1, "entry", "entry"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, output, _ := fixture(t)
			deployment, err := bundle.Load(output)
			must(t, err)
			profiles := model.Profiles{SchemaVersion: 1, TopologyID: deployment.TopologyID,
				Routes: map[string]model.Profile{"main": {Path: test.path, HopCount: test.hops, EntryNode: test.entry, ExitNode: test.exit}}}
			path := filepath.Join(output, "client-profiles.json")
			must(t, safeio.JSON(path, profiles))
			deployment.ProfilesSHA256, err = safeio.FileHash(path)
			must(t, err)
			if _, err := bundle.Dependencies(output, deployment); err == nil {
				t.Fatal("inconsistent route path accepted")
			}
		})
	}
}

func TestDeploymentDependenciesRejectChangedProfileBytes(t *testing.T) {
	_, output, _ := fixture(t)
	deployment, err := bundle.Load(output)
	must(t, err)
	path := filepath.Join(output, "client-profiles.json")
	raw, err := os.ReadFile(path)
	must(t, err)
	must(t, os.WriteFile(path, append(raw, '\n'), 0600))
	if _, err := bundle.Dependencies(output, deployment); err == nil {
		t.Fatal("changed profile bytes accepted")
	}
}
