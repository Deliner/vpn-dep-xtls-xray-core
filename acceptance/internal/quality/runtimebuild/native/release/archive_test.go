package release

import (
	"archive/zip"
	"bytes"
	"testing"
	"vpn-script/internal/platform/safeio"
)

func archiveFixture() (Policy, map[string][]byte) {
	members := map[string][]byte{"libcronet.so": []byte("reviewed library"), "source.tsv": []byte("reviewed source pins"),
		"build-provenance.txt": []byte("reviewed compiler"), "native-identity.txt": []byte("reviewed CPU/run"),
		"naive-evidence.sha256": []byte("reviewed Naive manifest"), "zlib-evidence.sha256": []byte("reviewed zlib manifest")}
	build := safeio.Hash(members["libcronet.so"]) + "  bin/libcronet.so\n" + safeio.Hash(members["source.tsv"]) + "  source.tsv\n" + safeio.Hash(members["build-provenance.txt"]) + "  provenance.txt\n"
	native := safeio.Hash(members["native-identity.txt"]) + "  identity.txt\n"
	members["build-evidence.sha256"], members["native-evidence.sha256"] = []byte(build), []byte(native)
	target := Target{Library: safeio.Hash(members["libcronet.so"]), Build: safeio.Hash([]byte(build)), Native: safeio.Hash([]byte(native)),
		Naive: safeio.Hash(members["naive-evidence.sha256"]), Zlib: safeio.Hash(members["zlib-evidence.sha256"])}
	return Policy{Sources: safeio.Hash(members["source.tsv"]), Targets: map[string]Target{"amd64": target}}, members
}

func TestArchiveRequiresReviewedLibraryAndEveryEvidenceMember(t *testing.T) {
	for _, damage := range []string{"", "libcronet.so", "source.tsv", "build-provenance.txt", "native-identity.txt", "build-evidence.sha256", "native-evidence.sha256", "naive-evidence.sha256", "zlib-evidence.sha256", "missing", "extra"} {
		t.Run(damage, func(t *testing.T) {
			policy, members := archiveFixture()
			switch damage {
			case "":
			case "missing":
				delete(members, "libcronet.so")
			case "extra":
				members["unreviewed"] = []byte("other")
			default:
				members[damage] = []byte("changed")
			}
			raw, err := archiveMembers(members)
			if err != nil {
				t.Fatal(err)
			}
			if err := verifyArchive(policy, "amd64", raw); (err == nil) != (damage == "") {
				t.Fatalf("damage %q: %v", damage, err)
			}
		})
	}
}

func TestArchiveReproducibilityAndUnsafeHeaders(t *testing.T) {
	policy, members := archiveFixture()
	first, err := archiveMembers(members)
	if err != nil {
		t.Fatal(err)
	}
	second, err := archiveMembers(members)
	if err != nil || !bytes.Equal(first, second) {
		t.Fatal("archive bytes are not reproducible", err)
	}
	for _, name := range []string{"../outside", "libcronet.so", "source.tsv"} {
		var output bytes.Buffer
		writer := zip.NewWriter(&output)
		for member, body := range members {
			if member == "build-provenance.txt" {
				member = name
			}
			header := &zip.FileHeader{Name: member, Method: zip.Deflate}
			header.SetMode(0644)
			if member == "libcronet.so" {
				header.SetMode(0755)
			}
			entry, err := writer.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write(body); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := verifyArchive(policy, "amd64", output.Bytes()); err == nil {
			t.Fatal("accepted unsafe header", name)
		}
	}
}
