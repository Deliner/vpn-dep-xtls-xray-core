package singrelease

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"vpn-script/internal/node/artifact"
)

func TestPackageIsReproducibleAndInstallerCompatible(t *testing.T) {
	f := newFixture(t)
	var archives [][]byte
	for _, name := range []string{"first", "second"} {
		output := filepath.Join(t.TempDir(), name)
		if err := Package(f.policy, f.input, output); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(output, archiveName(f.policy, f.input.Arch))
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		archives = append(archives, raw)
		if err := Package(f.policy, f.input, output); err == nil {
			t.Fatal("replaced existing package")
		}
		after, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(raw, after) {
			t.Fatal("existing package changed", err)
		}
	}
	if !bytes.Equal(archives[0], archives[1]) {
		t.Fatal("package is not reproducible")
	}
	prefix := archivePrefix(f.policy, f.input.Arch)
	paths, err := artifact.ExtractTarFiles(archives[0], t.TempDir(), prefix+"sing-box", prefix+"libcronet.so")
	if err != nil || len(paths) != 2 {
		t.Fatal("installer could not extract both runtime files", err)
	}
	for name, want := range map[string]string{"sing-box": "tested Go engine", "libcronet.so": "tested Cronet"} {
		raw, err := os.ReadFile(paths[name])
		if err != nil || string(raw) != want {
			t.Fatal("installed bytes differ", name, err)
		}
	}
}

func TestFailedVerificationLeavesNoPackage(t *testing.T) {
	f := newFixture(t)
	put(t, f.input.Candidate, "bin/libcronet.so", []byte("unreviewed companion"))
	output := filepath.Join(t.TempDir(), "output")
	if err := Package(f.policy, f.input, output); err == nil {
		t.Fatal("packaged an unreviewed library")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("failed verification created output", err)
	}
}
