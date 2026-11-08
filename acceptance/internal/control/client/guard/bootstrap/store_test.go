package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestStoreRejectsChangedAccessUnknownFilesAndConcurrentOwners(t *testing.T) {
	s, k := fixture(t, "")
	if second, err := openStore(s.base, false, k); err == nil {
		second.close()
		t.Fatal("two bootstrap owners acquired the same state")
	}
	require(t, safeio.Create(filepath.Join(s.base, "foreign"), []byte("keep"), 0600))
	if s.members() == nil {
		t.Fatal("unknown bootstrap file accepted")
	}
	require(t, os.Remove(filepath.Join(s.base, "foreign")))
	require(t, os.Chmod(filepath.Join(s.base, "lock"), 0644))
	if s.members() == nil {
		t.Fatal("public lock accepted")
	}
	require(t, os.Chmod(filepath.Join(s.base, "lock"), 0600))
	require(t, s.activate(t.Context()))
	require(t, os.Chmod(filepath.Join(s.base, "state.json"), 0644))
	if _, _, err := s.read(); err == nil {
		t.Fatal("public journal accepted")
	}
}

func TestJournalRejectsSymlinksAndUnknownSchemaWithoutKernelMutation(t *testing.T) {
	for _, content := range []string{`{"schema_version":2}`, `{"schema_version":1,"guard":{"token":"foreign"}}`} {
		t.Run(content, func(t *testing.T) {
			s, k := fixture(t, "")
			require(t, safeio.Create(filepath.Join(s.base, "state.json"), []byte(content), 0600))
			if s.activate(t.Context()) == nil || k.creations != 0 {
				t.Fatal("invalid journal reached kernel")
			}
		})
	}
	s, k := fixture(t, "")
	target := filepath.Join(t.TempDir(), "retained")
	require(t, safeio.Create(target, []byte("untouched"), 0600))
	require(t, os.Symlink(target, filepath.Join(s.base, "state.json")))
	if s.activate(t.Context()) == nil || k.creations != 0 {
		t.Fatal("linked journal accepted")
	}
	data, err := os.ReadFile(target)
	require(t, err)
	if string(data) != "untouched" {
		t.Fatal("linked target was changed")
	}
}
