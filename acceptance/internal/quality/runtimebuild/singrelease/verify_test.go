package singrelease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"vpn-script/internal/platform/safeio"
)

func TestReleaseAcceptsOnlyReviewedGenerations(t *testing.T) {
	for _, label := range []string{"nhop-fork-cronet154-zlib-1", tuicLabel, "unreviewed"} {
		f := newFixture(t)
		f.policy.Label = label
		if err := f.policy.valid(); (err == nil) != (label != "unreviewed") {
			t.Fatal(label, err)
		}
	}
}

func TestTUICReleaseRequiresDependencySourcesAndUpstreamAudit(t *testing.T) {
	for _, damage := range []string{"", "origin.txt", "sing-quic/LICENSE", "sing-quic/go.mod", "sing-quic/tuic/service.go",
		"sing-quic/tuic/server_listener.go", "version", "sum", "missing-audit", "duplicate-audit"} {
		t.Run(damage, func(t *testing.T) {
			f := newFixture(t)
			f.policy.Label = tuicLabel
			entries := tuicAttribution(t, f, damage)
			if err := verifyTUICCandidate(f.policy, f.input, entries); (err == nil) != (damage == "") {
				t.Fatal(damage, err)
			}
		})
	}
}

func tuicAttribution(t *testing.T, f fixture, damage string) map[string]string {
	t.Helper()
	entries := map[string]string{}
	for _, name := range []string{"origin.txt", "sing-quic/LICENSE", "sing-quic/go.mod", "sing-quic/tuic/service.go", "sing-quic/tuic/server_listener.go"} {
		if name != damage {
			entries["source/nhop-dependencies/"+name] = strings.Repeat("a", 64)
		}
	}
	version := "v0.7.2-0.20261020082049-7e0a5869a5f4"
	provenance := "tuic_module=github.com/sagernet/sing-quic\ntuic_version=" + version + "\n" +
		"tuic_upstream_sum=h1:QuhAb8QW0pKYmSvfj/nK91libunBwVJiWb852yidlxI=\n" +
		"tuic_source=source/nhop-dependencies/sing-quic\ntuic_patch_state=applied\n"
	if damage == "version" {
		provenance = strings.ReplaceAll(provenance, version, "unreviewed")
	}
	if damage == "sum" {
		provenance = strings.ReplaceAll(provenance, "h1:Quh", "h1:wrong")
	}
	audit := "Conservative upstream identity scan: github.com/sagernet/sing-quic@" + version + "\n"
	if damage == "missing-audit" {
		audit = "No vulnerabilities found.\n"
	}
	if damage == "duplicate-audit" {
		audit += audit
	}
	put(t, f.input.Candidate, "provenance.txt", []byte(provenance))
	put(t, f.input.Candidate, "audit/source.txt", []byte(audit))
	return entries
}

func TestReleaseRequiresCompleteNativeTestedCandidate(t *testing.T) {
	for _, name := range []string{"", "bin/sing-box", "bin/libcronet.so", "source/LICENSE", "source/go.sum", "source/extra.go", "bin/extra", "SHA256SUMS", "audit/source.txt", "audit/binary.txt", "audit/actual.sha256", "provenance.txt", "status"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			if name != "" {
				put(t, f.input.Candidate, name, []byte("changed"))
			}
			_, err := Verify(f.policy, f.input)
			if (err == nil) != (name == "") {
				t.Fatal(name, err)
			}
		})
	}
}

func TestReleaseRejectsChangedNativeEvidenceAndLinkedSources(t *testing.T) {
	for _, name := range []string{"host.txt", "status", "payload/report.json", "extra", "sing-box-SHA256SUMS", "sing-box-provenance.txt", "linked"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			if name == "linked" {
				path := filepath.Join(f.input.Candidate, "source/LICENSE")
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path+".original", path); err != nil {
					t.Fatal(err)
				}
			} else {
				put(t, f.input.Native, name, []byte("changed"))
			}
			if _, err := Verify(f.policy, f.input); err == nil {
				t.Fatal("accepted", name)
			}
		})
	}
}

func TestReviewedMetadataStillRequiresSuccessfulAuditAndRunIdentity(t *testing.T) {
	f := newFixture(t)
	raw := []byte("Scan incomplete.\nscanner_exit_status=1\n")
	put(t, f.input.Candidate, "audit/source.txt", raw)
	target := f.policy.Targets["arm64"]
	target.SourceAudit = safeio.Hash(raw)
	f.policy.Targets["arm64"] = target
	if _, err := Verify(f.policy, f.input); err == nil {
		t.Fatal("accepted unsuccessful audit with a matching hash")
	}
	for _, line := range []string{"run_id=124\n", "run_id=123\nrun_id=123\n", "run_id=1234\n"} {
		if err := requireLines([]byte(line), []string{"run_id=123"}); err == nil {
			t.Fatal("accepted differing or duplicate native identity")
		}
	}
}
