package safeio_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestFileHashMatchesCompleteContents(t *testing.T) {
	for _, size := range []int{0, 31, 32769, 2 << 20} {
		path := filepath.Join(t.TempDir(), "runtime.bin")
		data := bytes.Repeat([]byte{0x39, 0x5a, 0xe7}, (size+2)/3)[:size]
		if size > 0 {
			data[size-1] = 0x81
		}
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		expected := sha256.Sum256(data)
		actual, err := safeio.FileHash(path)
		if err != nil || actual != hex.EncodeToString(expected[:]) {
			t.Fatalf("hash of %d bytes: %s, %v", size, actual, err)
		}
	}
}

func TestFileHashSizeBoundary(t *testing.T) {
	for _, size := range []int64{150_000_000, 150_000_001} {
		path := sparseHashFile(t, size)
		digest, err := safeio.FileHash(path)
		if size == 150_000_000 && (err != nil || len(digest) != 64) {
			t.Fatalf("exact size limit rejected: %s, %v", digest, err)
		}
		if size > 150_000_000 && err == nil {
			t.Fatal("oversized file accepted")
		}
	}
}

func TestFileHashRejectsSymlinks(t *testing.T) {
	path := sparseHashFile(t, 1)
	link := filepath.Join(t.TempDir(), "linked.bin")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if _, err := safeio.FileHash(link); err == nil {
		t.Fatal("followed symlink while hashing")
	}
}

func BenchmarkFileHash(b *testing.B) {
	for _, size := range []int64{1 << 20, 32 << 20} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			path := sparseHashFile(b, size)
			b.SetBytes(size)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := safeio.FileHash(path); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func sparseHashFile(t testing.TB, size int64) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.bin")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(size); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
