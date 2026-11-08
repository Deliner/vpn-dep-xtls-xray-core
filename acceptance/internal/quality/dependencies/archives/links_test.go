package archives

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestArchivePreservesExactTrackedLinksWithoutFollowingThem(t *testing.T) {
	expected := map[string]fileIdentity{"bin/zstd": {SHA256: safeio.Hash([]byte("source")), Executable: true}, "bin/unzstd": {Symlink: "zstd"}}
	file := member{name: "repo/bin/zstd", body: "source", mode: 0755, typeflag: tar.TypeReg}
	link := member{name: "repo/bin/unzstd", mode: 0777, typeflag: tar.TypeSymlink, linkname: "zstd"}
	if err := verifyArchive(sourceFixture(t, file, link), "repo/", expected); err != nil {
		t.Fatal(err)
	}
	changed := link
	changed.linkname = "other"
	hard := link
	hard.typeflag = tar.TypeLink
	replaced := member{name: link.name, body: "zstd", mode: 0644, typeflag: tar.TypeReg}
	for _, invalid := range [][]member{{file}, {file, changed}, {file, hard}, {file, replaced}, {file, link, link}} {
		if err := verifyArchive(sourceFixture(t, invalid...), "repo/", expected); err == nil {
			t.Fatalf("accepted altered source symlink: %+v", invalid)
		}
	}
	links := sourceLinks(expected)
	if len(links) != 1 || links["bin/unzstd"] != "zstd" {
		t.Fatalf("pin lost the literal link target: %v", links)
	}
}

func TestSourceLinksRejectEscapeDirectoriesChainsAndGitlinkContents(t *testing.T) {
	for _, target := range []string{"/etc/passwd", "../../outside", "missing", ".", "alias", "link", "../submodule", "../submodule/file", "bad\nname", strings.Repeat("x", 4097)} {
		files := map[string]fileIdentity{"bin/target": {SHA256: "verified"}, "bin/alias": {Symlink: "target"},
			"bin/link": {Symlink: target}, "submodule": {Gitlink: strings.Repeat("a", 40)}}
		if err := validateSourceLinks(files); err == nil {
			t.Fatalf("accepted source link target %q", target)
		}
	}
	files := map[string]fileIdentity{"src/source.go": {SHA256: "verified"}, "bin/source": {Symlink: "../src/source.go"}}
	if err := validateSourceLinks(files); err != nil {
		t.Fatal("safe relative link to a tracked file was rejected", err)
	}
}

func TestSourceIdentityObservesLinkItselfAndChecksTrackedFileMode(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "source"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(directory, "link")); err != nil {
		t.Fatal(err)
	}
	identity, err := sourceIdentity(directory, "link", "120000", "")
	if err != nil || identity.Symlink != "missing" || identity.SHA256 != "" {
		t.Fatalf("link target was followed or lost: %+v %v", identity, err)
	}
	for _, pair := range [][2]string{{"link", "100644"}, {"source", "120000"}, {"source", "100755"}} {
		if _, err := sourceIdentity(directory, pair[0], pair[1], ""); err == nil {
			t.Fatalf("accepted replacement of Git file type/mode: %v", pair)
		}
	}
	identity, err = sourceIdentity(directory, "source", "100644", "")
	if err != nil || identity.SHA256 != safeio.Hash([]byte("source")) || identity.Executable {
		t.Fatalf("regular source was not verified: %+v %v", identity, err)
	}
}
