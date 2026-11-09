package attribution

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"vpn-script/internal/platform/safeio"
)

func sourceFixture(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, body := range files {
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return append(buffer.Bytes(), make([]byte, 2048)...)
}

func TestPackagePreservesCompleteReviewedTarAndReferencedLicenses(t *testing.T) {
	root := t.TempDir()
	raw := sourceFixture(t, map[string]string{"LICENSE": "root license", "src/vendor/README.chromium": "License File: //vendor/COPYING\n", "src/vendor/COPYING": "vendor terms", "src/code.c": "source bytes"})
	source, output := filepath.Join(root, "source.tar"), filepath.Join(root, "package")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Package(source, safeio.Hash(raw), output); err != nil {
		t.Fatal(err)
	}
	compressed, err := os.ReadFile(filepath.Join(output, "source.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(actual, raw) {
		t.Fatal("source tar or trailing bytes changed", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	notice, err := os.ReadFile(filepath.Join(output, "NOTICE.txt"))
	if err != nil || !strings.Contains(string(notice), "vendor terms") || strings.Contains(string(notice), "source bytes") {
		t.Fatal("notice selection differs", err)
	}
	if err := Package(source, safeio.Hash(raw), output); err == nil {
		t.Fatal("overwrote existing output")
	}
	after, err := os.ReadFile(filepath.Join(output, "source.tar.gz"))
	if err != nil || !bytes.Equal(compressed, after) {
		t.Fatal("existing output was damaged", err)
	}
}

func TestUnreviewedOrIncompleteSourcesLeaveNoPackage(t *testing.T) {
	for _, damage := range []string{"hash", "missing-root", "missing-reference", "traversal"} {
		t.Run(damage, func(t *testing.T) {
			files := map[string]string{"LICENSE": "root"}
			switch damage {
			case "missing-root":
				delete(files, "LICENSE")
			case "missing-reference":
				files["src/vendor/README.chromium"] = "License File: absent\n"
			case "traversal":
				files["../LICENSE"] = "outside"
			}
			raw := sourceFixture(t, files)
			sha := safeio.Hash(raw)
			if damage == "hash" {
				sha = strings.Repeat("0", 64)
			}
			root := t.TempDir()
			source, output := filepath.Join(root, "source.tar"), filepath.Join(root, "output")
			if err := os.WriteFile(source, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := Package(source, sha, output); err == nil {
				t.Fatal("accepted", damage)
			}
			if _, err := os.Lstat(output); !os.IsNotExist(err) {
				t.Fatal("failed package remains", err)
			}
		})
	}
}
