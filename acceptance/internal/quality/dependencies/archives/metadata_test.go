package archives

import (
	"archive/tar"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestArchiveRecordsGitlinksWithoutClaimingTheirSourceContents(t *testing.T) {
	expected := map[string]fileIdentity{"source.go": {SHA256: safeio.Hash([]byte("source"))}, "clients/android": {Gitlink: strings.Repeat("a", 40)}}
	files := []member{{name: "repo/source.go", body: "source", mode: 0644, typeflag: tar.TypeReg},
		{name: "repo/clients/", mode: 0755, typeflag: tar.TypeDir}, {name: "repo/clients/android/", mode: 0755, typeflag: tar.TypeDir}}
	if err := verifyArchive(sourceFixture(t, files...), "repo/", expected); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchive(sourceFixture(t, files[:1]...), "repo/", expected); err == nil {
		t.Fatal("omitted submodule boundary was accepted")
	}
	links := submodules(expected)
	if len(links) != 1 || links["clients/android"] != strings.Repeat("a", 40) {
		t.Fatalf("lost submodule commit: %v", links)
	}
	files = append(files, member{name: "repo/clients/android/injected.go", body: "injected", mode: 0644, typeflag: tar.TypeReg})
	if err := verifyArchive(sourceFixture(t, files...), "repo/", expected); err == nil {
		t.Fatal("accepted unverified submodule contents")
	}
}

func TestArchiveGlobalMetadataMustMatchTheSelectedCommit(t *testing.T) {
	revision := strings.Repeat("a", 40)
	header := &tar.Header{Name: "pax_global_header", Typeflag: tar.TypeXGlobalHeader, PAXRecords: map[string]string{"comment": revision}}
	if err := verifyArchiveComment(header, "repo-"+revision+"/"); err != nil {
		t.Fatal(err)
	}
	header.PAXRecords["comment"] = strings.Repeat("b", 40)
	if err := verifyArchiveComment(header, "repo-"+revision+"/"); err == nil {
		t.Fatal("accepted another commit in archive metadata")
	}
}

func TestSourceIndexRequiresTrackedRegularFilesOrExplicitGitlinks(t *testing.T) {
	for _, entry := range []string{"bad", "120000 abc 0\tsymlink", "100644 abc 1\tconflicted", "100644 abc 0\t../escape"} {
		if _, _, _, err := sourceEntry(entry); err == nil {
			t.Fatalf("accepted %q", entry)
		}
	}
	revision := strings.Repeat("a", 40)
	name, mode, object, err := sourceEntry("160000 " + revision + " 0\tclients/android")
	if err != nil || name != "clients/android" || mode != "160000" || object != revision {
		t.Fatalf("lost explicit Git link: %s %s %s %v", name, mode, object, err)
	}
}
