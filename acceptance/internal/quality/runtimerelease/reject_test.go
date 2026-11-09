package runtimerelease

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackagingRejectsUnverifiedInputsBeforeCreatingOutput(t *testing.T) {
	faults := map[string]string{
		"bin/xray": "changed executable", "source/core/start.go": "changed source",
		"source/unrecorded.go": "extra source", "bin/probe": "extra binary",
		"SHA256SUMS": "altered inventory", "status": "diagnostic-built\n",
		"provenance.txt":       "build_label=nhop-diagnostic-masque-4\n",
		"audit/actual.sha256":  "a different candidate",
		"audit/provenance.txt": "a different audit",
		"audit/source.txt":     "scanner_exit_status=1\n", "audit/binary.txt": "Scan not completed in this run.\n",
	}
	for name, contents := range faults {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			write(t, filepath.Join(f.input.Candidate, name), []byte(contents))
			reject(t, f)
		})
	}
	for _, name := range []string{"evidence.sha256", "host.txt", "status", "payload/1-hop.json", "xray-SHA256SUMS"} {
		t.Run("native/"+name, func(t *testing.T) {
			f := newFixture(t)
			write(t, filepath.Join(f.input.Native, name), []byte("altered native evidence"))
			reject(t, f)
		})
	}
}

func TestPackagingRejectsSymlinksEvenWithIdenticalBytes(t *testing.T) {
	for _, name := range []string{"source", "source/LICENSE", "bin/xray", "audit/source.txt"} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			path := filepath.Join(f.input.Candidate, name)
			moved := filepath.Join(f.root, "moved")
			if err := os.Rename(path, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(moved, path); err != nil {
				t.Fatal(err)
			}
			reject(t, f)
		})
	}
}

func TestInventoryRejectsDuplicateAndEscapingEntries(t *testing.T) {
	for _, name := range []string{"../escape", "/absolute", "source/../../escape", "source\\escape"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			raw := []byte(hash([]byte("x")) + "  " + name + "\n")
			write(t, filepath.Join(root, "SHA256SUMS"), raw)
			if _, err := inventory(root, "SHA256SUMS", hash(raw)); err == nil {
				t.Fatal("unsafe manifest path accepted")
			}
		})
	}
	root := t.TempDir()
	raw := writeInventory(t, root, "SHA256SUMS", map[string][]byte{"input": []byte("x")})
	raw = append(raw, raw...)
	write(t, filepath.Join(root, "SHA256SUMS"), raw)
	if _, err := inventory(root, "SHA256SUMS", hash(raw)); err == nil {
		t.Fatal("duplicate input accepted")
	}
}

func reject(t *testing.T, f fixture) {
	t.Helper()
	if err := Prepare(f.policy, f.input, f.output); err == nil {
		t.Fatal("unverified runtime accepted")
	}
	if _, err := os.Lstat(f.output); !os.IsNotExist(err) {
		t.Fatal("failed verification created release output", err)
	}
}
