package runtimebuild

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const outputDirectory = "build/runtime-candidates/test"

func TestSourcePinFailureStopsBeforeExtractionAndBuild(t *testing.T) {
	f := newFixture(t)
	data, err := os.ReadFile(f.archive)
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)/2] ^= 1
	write(t, f.archive, data, 0600)
	output, err := f.command("candidate.sh", "build", "xray", "linux", "amd64", outputDirectory).CombinedOutput()
	if err == nil {
		t.Fatalf("corrupt source accepted: %s", output)
	}
	entries, err := os.ReadDir(filepath.Join(f.root, outputDirectory, "source"))
	if err != nil || len(entries) != 0 {
		t.Fatal("unverified source was extracted")
	}
	if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
		t.Fatal("unverified source reached Go tooling")
	}
	assertStatus(t, f, "failed\n")
}

func TestBuildFailureLeavesDiagnosticState(t *testing.T) {
	f := newFixture(t)
	output, err := f.command("candidate.sh", "build", "xray", "linux", "amd64", outputDirectory).CombinedOutput()
	if err == nil {
		t.Fatalf("compiler failure ignored: %s", output)
	}
	trace, err := os.ReadFile(f.trace)
	if err != nil || !strings.Contains(string(trace), "build -trimpath -buildvcs=false") {
		t.Fatalf("verified source did not reach compiler: %v, %s", err, output)
	}
	assertStatus(t, f, "failed\n")
	if _, err := os.Stat(filepath.Join(f.root, outputDirectory, "provenance.txt")); !os.IsNotExist(err) {
		t.Fatal("failed build presented as completed candidate")
	}
}

func TestCandidateNeverOverwritesPreviousRun(t *testing.T) {
	f := newFixture(t)
	write(t, filepath.Join(f.root, outputDirectory, "status"), []byte("previous-run\n"), 0600)
	if output, err := f.command("candidate.sh", "build", "xray", "linux", "amd64", outputDirectory).CombinedOutput(); err == nil {
		t.Fatalf("existing run overwritten: %s", output)
	}
	assertStatus(t, f, "previous-run\n")
}

func TestCandidateRejectsSymlinkParentBeforeWriting(t *testing.T) {
	f := newFixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(f.root, "build")); err != nil {
		t.Fatal(err)
	}
	if output, err := f.command("candidate.sh", "build", "xray", "linux", "amd64", outputDirectory).CombinedOutput(); err == nil {
		t.Fatalf("symlink parent accepted: %s", output)
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatal("candidate wrote through parent symlink")
	}
}

func TestCandidateRejectsChangedPatchBeforeCompiler(t *testing.T) {
	f := newFixture(t)
	write(t, filepath.Join(f.root, "ops/runtime/xray/ephemeral-ports.patch"), []byte("unreviewed patch\n"), 0600)
	if output, err := f.command("candidate.sh", "build", "xray", "linux", "amd64", outputDirectory).CombinedOutput(); err == nil {
		t.Fatalf("unverified patch accepted: %s", output)
	}
	if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
		t.Fatal("unverified patch reached compiler")
	}
	assertStatus(t, f, "failed\n")
}

func TestCandidateRejectsChangedUpstreamPatchContext(t *testing.T) {
	f := newFixture(t)
	f.source(t, "upstream listener implementation changed\n")
	if output, err := f.command("candidate.sh", "build", "xray", "linux", "amd64", outputDirectory).CombinedOutput(); err == nil {
		t.Fatalf("patch with missing source context accepted: %s", output)
	}
	if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
		t.Fatal("unapplied source patch reached compiler")
	}
	assertStatus(t, f, "failed\n")
}

func TestCandidateAcceptsTheReviewedFixAlreadyInForkSources(t *testing.T) {
	f := newFixture(t)
	listener := "\t\t\t\t}\n\t\t\t}\n\n\t\t\t// A kernel-selected port must be exclusive: SO_REUSEPORT can assign\n\t\t\t// port zero to an existing UDP listener and misdeliver its packets.\n\t\t\t_, port, err := net.SplitHostPort(address)\n\t\t\tif err != nil || port != \"0\" {\n\t\t\t\tsetReusePort(fd)\n\t\t\t}\n\t\t})\n\t}\n}\n"
	f.source(t, listener)
	output, err := f.command("candidate.sh", "build", "xray", "linux", "amd64", outputDirectory).CombinedOutput()
	if err == nil {
		t.Fatalf("fixture compiler failure ignored: %s", output)
	}
	trace, err := os.ReadFile(f.trace)
	if err != nil || !strings.Contains(string(trace), "build -trimpath") {
		t.Fatalf("preapplied fix did not reach compiler: %v %s", err, output)
	}
	actual, err := os.ReadFile(filepath.Join(f.root, outputDirectory, "source/transport/internet/system_listener.go"))
	if err != nil || string(actual) != listener {
		t.Fatalf("preapplied patch changed source: %v", err)
	}
	metadata, err := os.ReadFile(filepath.Join(f.root, outputDirectory, "patch.txt"))
	if err != nil || !strings.Contains(string(metadata), "patch_state=already-in-source\n") {
		t.Fatalf("lost patch provenance: %s %v", metadata, err)
	}
}

func assertStatus(t *testing.T, f fixture, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.root, outputDirectory, "status"))
	if err != nil || string(data) != want {
		t.Fatalf("candidate status=%q, want %q: %v", data, want, err)
	}
}
