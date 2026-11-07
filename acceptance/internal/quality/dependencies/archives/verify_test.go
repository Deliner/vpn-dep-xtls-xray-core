package archives

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

	"vpn-script/internal/platform/safeio"
)

type member struct {
	name     string
	body     string
	mode     int64
	typeflag byte
}

func sourceFixture(t *testing.T, members ...member) []byte {
	t.Helper()
	var raw bytes.Buffer
	compressed := gzip.NewWriter(&raw)
	writer := tar.NewWriter(compressed)
	for _, m := range members {
		if err := writer.WriteHeader(&tar.Header{Name: m.name, Size: int64(len(m.body)), Mode: m.mode, Typeflag: m.typeflag}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(m.body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, close := range []func() error{writer.Close, compressed.Close} {
		if err := close(); err != nil {
			t.Fatal(err)
		}
	}
	return raw.Bytes()
}

func TestSourceArchiveRequiresExactFilesContentsAndExecutableBits(t *testing.T) {
	expected := map[string]fileIdentity{"source.go": {SHA256: safeio.Hash([]byte("source"))}}
	valid := member{name: "repo-revision/source.go", body: "source", mode: 0644, typeflag: tar.TypeReg}
	if err := verifyArchive(sourceFixture(t, valid), "repo-revision/", expected); err != nil {
		t.Fatal(err)
	}
	cases := [][]member{{}, {valid, valid}, {{name: "other/source.go", body: "source", mode: 0644}},
		{{name: valid.name, body: "modified", mode: 0644}}, {{name: valid.name, body: valid.body, mode: 0755}},
		{valid, {name: "repo-revision/extra.go", body: "extra", mode: 0644}},
		{{name: "repo-revision/../source.go", body: "source", mode: 0644}},
		{{name: valid.name, mode: 0644, typeflag: tar.TypeSymlink}}}
	for _, entries := range cases {
		if err := verifyArchive(sourceFixture(t, entries...), "repo-revision/", expected); err == nil {
			t.Fatalf("accepted changed source archive %+v", entries)
		}
	}
}

func TestSourceArchiveChecksGzipFooterAfterTarEOF(t *testing.T) {
	raw := sourceFixture(t, member{name: "repo/file", body: "source", mode: 0644, typeflag: tar.TypeReg})
	expected := map[string]fileIdentity{"file": {SHA256: safeio.Hash([]byte("source"))}}
	corrupt := append([]byte(nil), raw...)
	corrupt[len(corrupt)-8] ^= 1
	for _, invalid := range [][]byte{corrupt, raw[:len(raw)-5]} {
		if err := verifyArchive(invalid, "repo/", expected); err == nil {
			t.Fatal("accepted incomplete or corrupt gzip footer")
		}
	}
	if _, err := (tarPadding{}).Write([]byte("hidden second archive")); err == nil {
		t.Fatal("accepted hidden data after tar EOF")
	}
}
