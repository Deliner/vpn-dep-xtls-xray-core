package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stageFixture(t *testing.T) (Policy, string) {
	t.Helper()
	policy, members := archiveFixture()
	policy.Workflow, policy.Run = strings.Repeat("a", 40), "123"
	policy.Targets["arm64"] = policy.Targets["amd64"]
	root := t.TempDir()
	raw, err := archiveMembers(members)
	if err != nil {
		t.Fatal(err)
	}
	for _, arch := range []string{"amd64", "arm64"} {
		put(t, root, archiveName(arch), raw)
	}
	if err := sealStage(policy, root); err != nil {
		t.Fatal(err)
	}
	return policy, root
}

func TestDownloadedStageRejectsMissingLinkedAndRehashedSubstitution(t *testing.T) {
	for _, damage := range []string{"", "missing", "extra", "linked", "policy", "checksums", "rehashed-library"} {
		t.Run(damage, func(t *testing.T) {
			policy, root := stageFixture(t)
			switch damage {
			case "missing", "linked":
				if err := os.Remove(filepath.Join(root, archiveName("arm64"))); err != nil {
					t.Fatal(err)
				}
				if damage == "linked" {
					if err := os.Symlink(archiveName("amd64"), filepath.Join(root, archiveName("arm64"))); err != nil {
						t.Fatal(err)
					}
				}
			case "extra":
				put(t, root, "unreviewed", []byte("extra"))
			case "policy":
				put(t, root, "policy.json", []byte("{}\n"))
			case "checksums":
				put(t, root, "SHA256SUMS", []byte("other"))
			case "rehashed-library":
				_, members := archiveFixture()
				members["libcronet.so"] = []byte("substituted")
				raw, err := archiveMembers(members)
				if err != nil {
					t.Fatal(err)
				}
				put(t, root, archiveName("amd64"), raw)
				sums, err := stageSums(root)
				if err != nil {
					t.Fatal(err)
				}
				put(t, root, "SHA256SUMS", []byte(sums))
			}
			if err := VerifyPackage(policy, root); (err == nil) != (damage == "") {
				t.Fatalf("damage %q: %v", damage, err)
			}
		})
	}
}
