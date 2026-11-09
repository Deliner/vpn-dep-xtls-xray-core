package singrelease

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadedArchiveRejectsSubstitutionAndUnsafeStructure(t *testing.T) {
	f := newFixture(t)
	output := filepath.Join(t.TempDir(), "package")
	if err := Package(f.policy, f.input, output); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(output, archiveName(f.policy, f.input.Arch))
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"sing-box", "libcronet.so", "source/LICENSE", "source/go.mod", "NHOP-RELEASE.json", "missing", "duplicate", "symlink", "mode", "traversal", "gzip", "trailing"} {
		t.Run(fault, func(t *testing.T) {
			raw := damagedArchive(t, original, fault)
			put(t, output, "changed.tar.gz", raw)
			if err := VerifyArchive(f.policy, f.input.Arch, filepath.Join(output, "changed.tar.gz")); err == nil {
				t.Fatal("accepted", fault)
			}
		})
	}
}

func damagedArchive(t *testing.T, original []byte, fault string) []byte {
	t.Helper()
	compressed, err := gzip.NewReader(bytes.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	var output bytes.Buffer
	gzipWriter := gzip.NewWriter(&output)
	writer := tar.NewWriter(gzipWriter)
	copyDamagedMembers(t, reader, writer, fault)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if fault == "trailing" {
		if _, err := gzipWriter.Write([]byte("hidden payload")); err != nil {
			t.Fatal(err)
		}
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	raw := output.Bytes()
	if fault == "gzip" {
		raw[len(raw)-8] ^= 1
	}
	return raw
}

func copyDamagedMembers(t *testing.T, reader *tar.Reader, writer *tar.Writer, fault string) {
	t.Helper()
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		if filepath.Base(header.Name) == filepath.Base(fault) {
			raw = append(raw, '!')
			header.Size = int64(len(raw))
		}
		if filepath.Base(header.Name) == "libcronet.so" {
			if fault == "missing" {
				continue
			}
			damageHeader(header, fault)
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(raw[:header.Size]); err != nil {
			t.Fatal(err)
		}
		if fault == "duplicate" && filepath.Base(header.Name) == "libcronet.so" {
			if err := archiveMember(writer, header.Name, raw, header.Mode); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func damageHeader(header *tar.Header, fault string) {
	switch fault {
	case "symlink":
		header.Typeflag, header.Linkname, header.Size = tar.TypeSymlink, "outside", 0
	case "mode":
		header.Mode = 0644
	case "traversal":
		header.Name = "../outside"
	}
}
