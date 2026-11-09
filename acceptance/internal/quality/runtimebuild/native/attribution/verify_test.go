package attribution

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"vpn-script/internal/platform/safeio"
)

func TestDownloadedSourcesRequireReviewedTarAndCompleteNotices(t *testing.T) {
	root := t.TempDir()
	raw := sourceFixture(t, map[string]string{"LICENSE": "root", "vendor/NOTICE": "vendor terms"})
	source, output := filepath.Join(root, "source.tar"), filepath.Join(root, "output")
	if err := os.WriteFile(source, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sha := safeio.Hash(raw)
	if err := Package(source, sha, output); err != nil {
		t.Fatal(err)
	}
	archive, notice := filepath.Join(output, "source.tar.gz"), filepath.Join(output, "NOTICE.txt")
	if err := Verify(archive, notice, sha); err != nil {
		t.Fatal(err)
	}
	if err := Verify(archive, notice, strings.Repeat("0", 64)); err == nil {
		t.Fatal("accepted source substitution")
	}
	if err := os.WriteFile(notice, []byte("root"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(archive, notice, sha); err == nil {
		t.Fatal("accepted incomplete notices")
	}
}

func TestDownloadedSourcesRejectCorruptionAndSymlinks(t *testing.T) {
	for _, damage := range []string{"crc", "truncated", "trailing", "symlink"} {
		t.Run(damage, func(t *testing.T) {
			root := t.TempDir()
			raw := sourceFixture(t, map[string]string{"LICENSE": "root"})
			source, output := filepath.Join(root, "source.tar"), filepath.Join(root, "output")
			if err := os.WriteFile(source, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := Package(source, safeio.Hash(raw), output); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(output, "source.tar.gz")
			damageArchive(t, archive, damage)
			if err := Verify(archive, filepath.Join(output, "NOTICE.txt"), safeio.Hash(raw)); err == nil {
				t.Fatal("accepted", damage)
			}
		})
	}
}

func damageArchive(t *testing.T, archive, damage string) {
	t.Helper()
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	switch damage {
	case "crc":
		raw[len(raw)-8] ^= 1
	case "truncated":
		raw = raw[:len(raw)-4]
	case "trailing":
		raw = append(raw, []byte("not a gzip stream")...)
	case "symlink":
		if err := os.Rename(archive, archive+".original"); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(archive+".original", archive); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.WriteFile(archive, raw, 0600); err != nil {
		t.Fatal(err)
	}
}
