package network

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/runtime/xray"
)

func fixture(t *testing.T) profile.Imported {
	t.Helper()
	config := xray.Client(model.Object{"protocol": "shadowsocks", "tag": "proxy", "settings": model.Object{}}, 12080)
	raw, err := safeio.Encode(config)
	if err != nil {
		t.Fatal(err)
	}
	value := profile.Imported{SchemaVersion: 1, Topology: "fixture", Route: "main", Metadata: model.Profile{Engine: "xray", RuntimeVersion: xray.Version,
		Path: []string{"entry.proxy"}, HopCount: 1, EntryNode: "entry", ExitNode: "entry", SOCKSPort: 12080,
		EntryEndpoint: model.ClientEndpoint{Address: "198.51.100.7", Port: 24443, Network: "tcp,udp"}}}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value.Config); err != nil {
		t.Fatal(err)
	}
	value.Metadata.ConfigSHA256, err = safeio.Digest(value.Config)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestFirewallHasNoEstablishedOrSharedEntryBypass(t *testing.T) {
	policy, err := Firewall(fixture(t), Defaults(1234))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"established", "flush ruleset", "policy accept", "dport 53 accept"} {
		if strings.Contains(policy, forbidden) {
			t.Fatalf("unrestricted egress in kill switch: %s", forbidden)
		}
	}
	for _, line := range strings.Split(policy, "\n") {
		if strings.Contains(line, "198.51.100.7") && !strings.Contains(line, "meta skuid 1234") {
			t.Fatal("entry exception available to every application")
		}
	}
}

func TestNetworkPlansRejectUnvalidatedInterpolation(t *testing.T) {
	for _, mutate := range []func(*Options){
		func(o *Options) { o.Interface = "lo\" accept;" },
		func(o *Options) { o.EngineUID = 0 },
		func(o *Options) { o.IPv6 = "direct" },
		func(o *Options) { o.DNSIP = "resolver.example.invalid" },
	} {
		options := Defaults(1234)
		mutate(&options)
		if _, err := Firewall(fixture(t), options); err == nil {
			t.Fatal("invalid network option reached nft program")
		}
		if _, err := TUN(fixture(t), options); err == nil {
			t.Fatal("invalid network option reached TUN config")
		}
	}
}
