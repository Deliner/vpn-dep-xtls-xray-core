package profile

import (
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/core/protocol"
	"vpn-script/internal/platform/safeio"
)

func TestAllGeneratedProtocolsImportWithCustomPortsAndEntryMetadata(t *testing.T) {
	count := 0
	for _, spec := range protocol.All() {
		if spec.Status != "implemented" {
			continue
		}
		count++
		t.Run(spec.ID, func(t *testing.T) {
			value, err := Import(exportFixture(t, spec.ID), "main")
			if err != nil {
				t.Fatal(err)
			}
			e := value.Metadata.EntryEndpoint
			if e.Address != "198.51.100.7" || e.Port != 24443 || e.Network != spec.Network || value.Metadata.Engine != spec.Engine || value.Metadata.SOCKSPort != 12080 {
				t.Fatal("generated route lost its explicit transport or custom ports")
			}
		})
	}
	if count != 23 {
		t.Fatalf("protocol coverage changed: %d", count)
	}
}

func TestImportRefusesTraversalAndSymlinkConfiguration(t *testing.T) {
	directory := exportFixture(t, "shadowsocks-2022")
	if _, err := Import(directory, "../main"); err == nil {
		t.Fatal("route traversal accepted")
	}
	path := filepath.Join(directory, "clients/main.json")
	if err := os.Rename(path, path+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path+".saved", path); err != nil {
		t.Fatal(err)
	}
	if _, err := Import(directory, "main"); err == nil {
		t.Fatal("configuration symlink accepted")
	}
}

func TestImportedProfileRejectsChangedPayloadAndPublicSOCKS(t *testing.T) {
	value, err := Import(exportFixture(t, "shadowsocks-2022"), "main")
	if err != nil {
		t.Fatal(err)
	}
	value.Config["inbounds"].([]any)[0].(map[string]any)["listen"] = "0.0.0.0"
	if Validate(value) == nil {
		t.Fatal("changed payload accepted")
	}
	value.Metadata.ConfigSHA256, err = safeio.Digest(value.Config)
	if err != nil {
		t.Fatal(err)
	}
	if Validate(value) == nil {
		t.Fatal("public SOCKS proxy accepted even with matching digest")
	}
}
