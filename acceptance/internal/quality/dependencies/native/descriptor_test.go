package native

import (
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestDescriptorPreservesDeclarationsWithoutInferringApplicability(t *testing.T) {
	raw := []byte("Name: Library\nURL: https://example.org/src\nRevision: abc123\nShipped: no\nSecurity Critical: Yes\nCPEPrefix: unknown\n")
	descriptor, err := parseDescriptor("src/third_party/library/README.chromium", raw)
	if err != nil || descriptor.SHA256 != safeio.Hash(raw) || len(descriptor.Components) != 1 {
		t.Fatalf("descriptor=%+v error=%v", descriptor, err)
	}
	fields := descriptor.Components[0]
	if fields["CPEPrefix"] != "unknown" || fields["Security Critical"] != "Yes" || fields["Shipped"] != "no" || fields["Revision"] != "abc123" {
		t.Fatal("parser inferred security applicability from upstream metadata")
	}
}

func TestDescriptorRefusesToOverwriteASecondComponent(t *testing.T) {
	if _, err := parseDescriptor("multi", []byte("Name: first\nName: second\n")); err == nil {
		t.Fatal("silently lost a component")
	}
	descriptor, err := parseDescriptor("fixture", []byte("This file is a license test fixture.\n"))
	if err != nil || len(descriptor.Components) != 1 || len(descriptor.Components[0]) != 0 {
		t.Fatalf("noncomponent descriptor was discarded or invented: %+v, %v", descriptor, err)
	}
}

func TestDescriptorKeepsSeparateComponentsAndTheirRevisions(t *testing.T) {
	raw := []byte("Name: Darwin\nRevision: abc123\n-------------------- DEPENDENCY DIVIDER --------------------\nName: CF\nVersion: CF-635\n")
	descriptor, err := parseDescriptor("apple/README.chromium", raw)
	if err != nil || len(descriptor.Components) != 2 {
		t.Fatalf("descriptor=%+v error=%v", descriptor, err)
	}
	if descriptor.Components[0]["Name"] != "Darwin" || descriptor.Components[1]["Name"] != "CF" || descriptor.Components[1]["Revision"] != "" {
		t.Fatalf("mixed independent component provenance: %+v", descriptor.Components)
	}
}

func TestDescriptorPathsRefuseTraversalAndKeepNestedSources(t *testing.T) {
	for _, raw := range []string{"", "../README.chromium\x00", "/README.chromium\x00", "a/../README.chromium\x00"} {
		if _, err := descriptorPaths([]byte(raw)); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	paths, err := descriptorPaths([]byte("README.md\x00src/third_party/README.chromium\x00"))
	if err != nil || len(paths) != 1 || paths[0] != "src/third_party/README.chromium" {
		t.Fatalf("paths=%q error=%v", paths, err)
	}
}
