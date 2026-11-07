package artifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tarArchive(t *testing.T, kind byte, duplicate bool) []byte {
	t.Helper()
	var raw bytes.Buffer
	compressed := gzip.NewWriter(&raw)
	archive := tar.NewWriter(compressed)
	count := 1
	if duplicate {
		count = 2
	}
	for range count {
		header := &tar.Header{Name: "release/sing-box", Typeflag: kind, Mode: 0755, Size: 3, Linkname: "/outside"}
		if kind != tar.TypeReg {
			header.Size = 0
		}
		require(t, archive.WriteHeader(header))
		if kind == tar.TypeReg {
			_, err := archive.Write([]byte("ELF"))
			require(t, err)
		}
	}
	require(t, archive.WriteHeader(&tar.Header{Name: "../escape", Typeflag: tar.TypeReg, Mode: 0600}))
	require(t, archive.Close())
	require(t, compressed.Close())
	return raw.Bytes()
}

func TestTarExtractsOnlyExpectedRegularMember(t *testing.T) {
	directory := t.TempDir()
	path, err := ExtractTar(tarArchive(t, tar.TypeReg, false), directory, "release/sing-box")
	require(t, err)
	raw, err := os.ReadFile(path)
	require(t, err)
	if string(raw) != "ELF" {
		t.Fatal("incorrect extraction")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(directory), "escape")); !os.IsNotExist(err) {
		t.Fatal("archive escaped destination")
	}
	for _, kind := range []byte{tar.TypeSymlink, tar.TypeLink, tar.TypeDir} {
		if _, err := ExtractTar(tarArchive(t, kind, false), directory, "release/sing-box"); err == nil {
			t.Fatal("special archive entry accepted")
		}
	}
	if _, err := ExtractTar(tarArchive(t, tar.TypeReg, true), directory, "release/sing-box"); err == nil {
		t.Fatal("duplicate member accepted")
	}
}

type tarEntry struct {
	name string
	kind byte
	data string
}

var tarPair = []tarEntry{
	{"release/sing-box", tar.TypeReg, "ELF engine contents"},
	{"release/libcronet.so", tar.TypeReg, "ELF companion contents"},
}

func tarBundle(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var raw bytes.Buffer
	compressed := gzip.NewWriter(&raw)
	archive := tar.NewWriter(compressed)
	for _, entry := range entries {
		header := &tar.Header{Name: entry.name, Mode: 0755, Typeflag: entry.kind, Linkname: "/outside"}
		if entry.kind == tar.TypeReg {
			header.Size = int64(len(entry.data))
		}
		require(t, archive.WriteHeader(header))
		if entry.kind == tar.TypeReg {
			_, err := io.WriteString(archive, entry.data)
			require(t, err)
		}
	}
	require(t, archive.Close())
	require(t, compressed.Close())
	return raw.Bytes()
}

func TestTarFilesExtractCompletePairWithExactBytesAndPermissions(t *testing.T) {
	directory := t.TempDir()
	entries := append(append([]tarEntry{}, tarPair...), tarEntry{"../escape", tar.TypeReg, "ignored"})
	paths, err := ExtractTarFiles(tarBundle(t, entries), directory, tarPair[1].name, tarPair[0].name)
	require(t, err)
	for _, entry := range tarPair {
		name := filepath.Base(entry.name)
		path := paths[name]
		if path != filepath.Join(directory, name) {
			t.Fatal("incorrect output path")
		}
		raw, err := os.ReadFile(path)
		require(t, err)
		info, err := os.Stat(path)
		require(t, err)
		if string(raw) != entry.data || info.Mode().Perm() != 0755 {
			t.Fatal("member bytes or mode differ")
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(directory), "escape")); !os.IsNotExist(err) {
		t.Fatal("unselected archive path escaped the stage")
	}
}

func TestInvalidTarPairDoesNotPublishEitherFile(t *testing.T) {
	valid := tarBundle(t, tarPair)
	corrupted := bytes.Clone(valid)
	corrupted[len(corrupted)-8] ^= 1
	cases := map[string][]byte{
		"missing-companion": tarBundle(t, tarPair[:1]),
		"duplicate-engine":  tarBundle(t, append(append([]tarEntry{}, tarPair...), tarPair[0])),
		"duplicate-library": tarBundle(t, append(append([]tarEntry{}, tarPair...), tarPair[1])),
		"linked-library":    tarBundle(t, []tarEntry{tarPair[0], {tarPair[1].name, tar.TypeSymlink, ""}}),
		"linked-engine":     tarBundle(t, []tarEntry{{tarPair[0].name, tar.TypeLink, ""}, tarPair[1]}),
		"checksum":          corrupted,
		"truncated-gzip":    valid[:len(valid)-4],
	}
	for name, archive := range cases {
		t.Run(name, func(t *testing.T) { rejectTarWithoutPublication(t, archive) })
	}
}

func rejectTarWithoutPublication(t *testing.T, archive []byte) error {
	t.Helper()
	directory := t.TempDir()
	for _, entry := range tarPair {
		require(t, os.WriteFile(filepath.Join(directory, filepath.Base(entry.name)), []byte("previous"), 0600))
	}
	_, extractErr := ExtractTarFiles(archive, directory, tarPair[0].name, tarPair[1].name)
	if extractErr == nil {
		t.Fatal("invalid runtime pair accepted")
	}
	for _, entry := range tarPair {
		path := filepath.Join(directory, filepath.Base(entry.name))
		raw, err := os.ReadFile(path)
		require(t, err)
		info, err := os.Stat(path)
		require(t, err)
		if string(raw) != "previous" || info.Mode().Perm() != 0600 {
			t.Fatal("failed archive replaced a prior file")
		}
	}
	entries, err := os.ReadDir(directory)
	require(t, err)
	if len(entries) != 2 {
		t.Fatal("failed archive left partial extraction files")
	}
	return extractErr
}

func TestTarSelectionRejectsAmbiguousAndUnexpectedTargets(t *testing.T) {
	for _, members := range [][]string{nil, {"other"}, {"a/sing-box", "b/sing-box"}, {"a/libcronet.so", "a/libcronet.so"}, {"sing-box", "libcronet.so", "other"}} {
		directory := t.TempDir()
		if _, err := ExtractTarFiles(tarBundle(t, tarPair), directory, members...); err == nil {
			t.Fatalf("invalid selection accepted: %v", members)
		}
		entries, err := os.ReadDir(directory)
		require(t, err)
		if len(entries) != 0 {
			t.Fatal("invalid selection published files")
		}
	}
}

func TestTarResourceBoundIncludesBytesAfterTarEOF(t *testing.T) {
	reader, err := gzip.NewReader(bytes.NewReader(tarBundle(t, tarPair)))
	require(t, err)
	plain, err := io.ReadAll(reader)
	require(t, err)
	require(t, reader.Close())
	plain = append(plain, make([]byte, 1024)...)
	var encoded bytes.Buffer
	writer := gzip.NewWriter(&encoded)
	_, err = writer.Write(plain)
	require(t, err)
	require(t, writer.Close())
	for _, limit := range []int64{int64(len(plain) - 1), int64(len(plain))} {
		selection := map[string][]byte{tarPair[0].name: nil, tarPair[1].name: nil}
		err := unpackTar(encoded.Bytes(), selection, limit)
		if (err == nil) != (limit == int64(len(plain))) {
			t.Fatalf("incorrect decompression bound at %d: %v", limit, err)
		}
	}
}

func TestTarRejectsOversizedDeclarationBeforeReadingBody(t *testing.T) {
	var raw bytes.Buffer
	compressed := gzip.NewWriter(&raw)
	archive := tar.NewWriter(compressed)
	require(t, archive.WriteHeader(&tar.Header{Name: tarPair[0].name, Mode: 0755, Size: 150_000_000}))
	// Deliberately omit the declared body and tar terminator.
	require(t, compressed.Close())
	if err := rejectTarWithoutPublication(t, raw.Bytes()); !strings.Contains(err.Error(), "unsafe runtime tar member") {
		t.Fatalf("oversized header reached body processing: %v", err)
	}
}
