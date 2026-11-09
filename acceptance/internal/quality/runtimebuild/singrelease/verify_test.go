package singrelease

import (
	"os"
	"path/filepath"
	"testing"
	"vpn-script/internal/platform/safeio"
)

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
