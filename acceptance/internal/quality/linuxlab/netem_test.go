package linuxlab

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func netemFixture(t *testing.T) fixture {
	t.Helper()
	f := newFixture(t)
	data, err := os.ReadFile("../../../ops/testing/diagnostics/netem/runtime.sh")
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(f.root, "ops/testing/diagnostics/netem/runtime.sh"), data)
	path := filepath.Join(f.root, "build/runtime-candidates/xray candidate")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(f.xray, path); err != nil {
		t.Fatal(err)
	}
	f.xray, f.output = path, filepath.Join(f.root, "build/netem input")
	if err := os.Mkdir(f.output, 0700); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f fixture) netemCommand(ctx context.Context) *exec.Cmd {
	return process.Command(ctx, "bash", "-c", `source "$1"; netem_prepare_runtime "$2" "$3" arm64 "$4"`, "netem-fixture",
		filepath.Join(f.root, "ops/testing/diagnostics/netem/runtime.sh"), f.root, f.xray, f.output)
}

func TestNetemCandidatePreservesOrdinaryRuntimeIdentity(t *testing.T) {
	f := netemFixture(t)
	command := f.netemCommand(t.Context())
	defer process.Stop(command)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("valid candidate rejected: %v, %s", err, output)
	}
	kind, err := os.ReadFile(filepath.Join(f.output, "runtime-kind"))
	if err != nil || string(kind) != "runtime-candidate\n" {
		t.Fatal("ordinary candidate misrepresented as a diagnostic probe")
	}
	for _, file := range []string{"control.log", "probe.json"} {
		if _, err := os.Stat(filepath.Join(f.output, file)); !os.IsNotExist(err) {
			t.Fatal("ordinary candidate invented probe evidence", file)
		}
	}
	for source, copy := range map[string]string{"bin/xray": "bin/xray", "SHA256SUMS": "runtime.inventory.sha256", "provenance.txt": "provenance.txt"} {
		expected, err := os.ReadFile(filepath.Join(f.xray, source))
		if err != nil {
			t.Fatal(err)
		}
		actual, err := os.ReadFile(filepath.Join(f.output, copy))
		if err != nil || string(actual) != string(expected) {
			t.Fatal("candidate identity changed during packaging", source)
		}
	}
}

func TestNetemRejectsUnverifiedCandidateBeforeCopy(t *testing.T) {
	for _, kind := range []string{"binary", "source", "extra", "incomplete", "wrong-engine", "wrong-target", "fifo", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			f := netemFixture(t)
			tamperNetemCandidate(t, f, kind)
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			command := f.netemCommand(ctx)
			defer process.Stop(command)
			output, err := command.CombinedOutput()
			if err == nil || ctx.Err() != nil {
				t.Fatalf("invalid candidate accepted or blocked: %v, %s", err, output)
			}
			if _, err := os.Stat(filepath.Join(f.output, "bin/xray")); !os.IsNotExist(err) {
				t.Fatal("unverified candidate reached binary copy")
			}
		})
	}
}

func tamperNetemCandidate(t *testing.T, f fixture, kind string) {
	t.Helper()
	switch kind {
	case "binary":
		write(t, filepath.Join(f.xray, "bin/xray"), []byte("modified"))
	case "source":
		write(t, filepath.Join(f.xray, "source/source.txt"), []byte("modified"))
	case "extra":
		write(t, filepath.Join(f.xray, "source/extra"), []byte("unrecorded"))
	case "incomplete":
		write(t, filepath.Join(f.xray, "status"), []byte("building\n"))
	case "wrong-engine":
		write(t, filepath.Join(f.xray, "provenance.txt"), []byte("engine=sing-box\ntarget=linux/arm64\n"))
	case "wrong-target":
		write(t, filepath.Join(f.xray, "provenance.txt"), []byte("engine=xray\ntarget=darwin/arm64\n"))
	case "fifo":
		if err := syscall.Mkfifo(filepath.Join(f.xray, "source/fifo"), 0600); err != nil {
			t.Fatal(err)
		}
	case "symlink":
		if err := os.Remove(filepath.Join(f.xray, "bin/xray")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(f.xray, "source/source.txt"), filepath.Join(f.xray, "bin/xray")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNetemProbeRetainsObservationEvidence(t *testing.T) {
	f := netemFixture(t)
	path := filepath.Join(f.root, "build/runtime-probes/diagnostic")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(f.xray, path); err != nil {
		t.Fatal(err)
	}
	f.xray = path
	write(t, filepath.Join(path, "status"), []byte("diagnostic-built\n"))
	write(t, filepath.Join(path, "control.log"), []byte("verified controls\n"))
	write(t, filepath.Join(path, "probe.json"), []byte("{\"kind\":\"diagnostic-only\"}\n"))
	var inventory strings.Builder
	for _, name := range []string{"bin/xray", "provenance.txt", "control.log", "probe.json"} {
		data, err := os.ReadFile(filepath.Join(path, name))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&inventory, "%x  %s\n", sha256.Sum256(data), name)
	}
	write(t, filepath.Join(path, "SHA256SUMS"), []byte(inventory.String()))
	command := f.netemCommand(t.Context())
	defer process.Stop(command)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("valid probe rejected: %v, %s", err, output)
	}
	kind, err := os.ReadFile(filepath.Join(f.output, "runtime-kind"))
	if err != nil || string(kind) != "diagnostic-probe\n" {
		t.Fatal("probe observations were lost")
	}
	for _, name := range []string{"control.log", "probe.json"} {
		if _, err := os.Stat(filepath.Join(f.output, name)); err != nil {
			t.Fatal("probe evidence missing", name, err)
		}
	}
}
