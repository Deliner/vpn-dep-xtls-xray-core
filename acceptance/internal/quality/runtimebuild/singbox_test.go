package runtimebuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSingBoxConfigurationPreservesFullUpstreamFeatures(t *testing.T) {
	f := newFixture(t)
	tags := "with_gvisor,with_naive_outbound,with_openvpn,with_openconnect"
	work := filepath.Join(f.root, "work")
	write(t, filepath.Join(work, "source/release/DEFAULT_BUILD_TAGS"), []byte(tags), 0600)
	write(t, filepath.Join(work, "source/release/LDFLAGS"), []byte("-checklinkname=0"), 0600)
	script := `source "$1"; candidate_tuic_prepare() { :; }; candidate_cronet_prepare() { :; }; candidate_cronet_release() { :; }; candidate_configure; printf '%s\n%s\n%s\n%s\n' "$build_package" "$build_tags" "$build_cgo" "$build_flags"`
	command := exec.Command("bash", "-eu", "-c", script, "fixture", filepath.Join(f.root, "ops/runtime/sing-box/build.sh"))
	command.Env = append(os.Environ(), "work="+work, "target_os=linux", "version=1.14.4", "recipe="+filepath.Join(f.root, "ops/runtime/sing-box"))
	output, err := command.CombinedOutput()
	want := "./cmd/sing-box\n" + tags + ",nhop_fork_cronet154_1,with_purego\n0\n"
	if err != nil || !strings.HasPrefix(string(output), want) || !strings.Contains(string(output), "-checklinkname=0") {
		t.Fatalf("upstream build contract changed: %v, %s", err, output)
	}
}

func TestSingBoxAuditUsesRecordedTagsAndCompanionInventory(t *testing.T) {
	f := auditFixture(t, "sing-box")
	if output, err := f.command("audit-candidate.sh", outputDirectory).CombinedOutput(); err != nil {
		t.Fatalf("sing-box fixture scan failed: %v, %s", err, output)
	}
	trace, err := os.ReadFile(f.trace)
	if err != nil || !strings.Contains(string(trace), "-tags with_gvisor,with_naive_outbound,with_purego") || !strings.Contains(string(trace), "./cmd/sing-box") {
		t.Fatalf("source audit omitted compiled features: %v, %s", err, trace)
	}
	if !strings.Contains(string(trace), "CGO_ENABLED=0 -mode binary "+filepath.Join(f.root, outputDirectory, "bin/sing-box")) {
		t.Fatalf("binary audit did not select sing-box: %s", trace)
	}
	if err := os.Remove(f.trace); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(f.root, outputDirectory, "bin/libcronet.so"), []byte("changed library"), 0600)
	if output, err := f.command("audit-candidate.sh", outputDirectory).CombinedOutput(); err == nil {
		t.Fatalf("changed companion accepted: %s", output)
	}
	if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
		t.Fatal("scanner ran with an unverified companion")
	}
}
