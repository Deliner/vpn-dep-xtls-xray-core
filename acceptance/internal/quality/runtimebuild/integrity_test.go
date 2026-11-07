package runtimebuild

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func boundedCommand(t *testing.T, command *exec.Cmd) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	bounded := process.Command(ctx, command.Args[0], command.Args[1:]...)
	bounded.Dir, bounded.Env = command.Dir, command.Env
	output, err := bounded.CombinedOutput()
	_ = process.Stop(bounded)
	return output, err
}

func TestCandidateAuditRejectsSpecialFilesBeforeScanning(t *testing.T) {
	for _, kind := range []string{"fifo-source", "fifo-bin", "socket"} {
		t.Run(kind, func(t *testing.T) {
			f := auditFixture(t, "sing-box")
			path := filepath.Join(f.root, outputDirectory, "bin/unexpected")
			if kind == "fifo-source" {
				path = filepath.Join(f.root, outputDirectory, "source/unexpected")
			}
			addSpecialFile(t, path, kind)
			output, err := boundedCommand(t, f.command("audit-candidate.sh", outputDirectory))
			if err == nil || !strings.Contains(string(output), "non-regular") {
				t.Fatalf("special input was not rejected explicitly: %v, %s", err, output)
			}
			if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
				t.Fatal("scanner ran despite a special input")
			}
		})
	}
}

func addSpecialFile(t *testing.T, path, kind string) {
	t.Helper()
	if kind != "socket" {
		if err := syscall.Mkfifo(path, 0600); err != nil {
			t.Fatal(err)
		}
		return
	}
	// A relative bind avoids the short Unix socket path limit in macOS test directories.
	t.Chdir(filepath.Dir(path))
	listener, err := net.Listen("unix", filepath.Base(path))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
}

func TestCandidateMetadataCannotBeFIFO(t *testing.T) {
	for _, name := range []string{"status", "provenance.txt", "SHA256SUMS"} {
		t.Run(name, func(t *testing.T) {
			f := auditFixture(t, "xray")
			path := filepath.Join(f.root, outputDirectory, name)
			for _, scan := range []string{"source", "binary"} {
				write(t, filepath.Join(f.root, outputDirectory, "audit", scan+".txt"), []byte("old success"), 0600)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			addSpecialFile(t, path, "fifo")
			output, err := boundedCommand(t, f.command("audit-candidate.sh", outputDirectory))
			if err == nil || !strings.Contains(string(output), "regular metadata") {
				t.Fatalf("unsafe metadata not rejected before read: %v, %s", err, output)
			}
			if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
				t.Fatal("scanner ran with unsafe metadata")
			}
			for _, scan := range []string{"source", "binary"} {
				data, err := os.ReadFile(filepath.Join(f.root, outputDirectory, "audit", scan+".txt"))
				if err != nil || !strings.Contains(string(data), "not completed") {
					t.Fatalf("stale scan survives metadata failure: %v, %s", err, data)
				}
			}
		})
	}
}

func TestAuditOutputsCannotBlockOnFIFO(t *testing.T) {
	for _, name := range []string{"source.txt", "binary.txt", "actual.sha256", "scanner.txt", "provenance.txt"} {
		t.Run(name, func(t *testing.T) {
			f := auditFixture(t, "xray")
			path := filepath.Join(f.root, outputDirectory, "audit", name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			addSpecialFile(t, path, "fifo")
			output, err := boundedCommand(t, f.command("audit-candidate.sh", outputDirectory))
			if err == nil || !strings.Contains(string(output), "regular file") {
				t.Fatalf("unsafe output not rejected before write: %v, %s", err, output)
			}
			if _, err := os.Stat(f.trace); !os.IsNotExist(err) {
				t.Fatal("scanner ran with unsafe outputs")
			}
		})
	}
}

func TestAuditDoesNotWriteThroughOutputSymlink(t *testing.T) {
	for _, name := range []string{"audit", "audit/source.txt"} {
		t.Run(name, func(t *testing.T) {
			f := auditFixture(t, "xray")
			target := filepath.Join(f.root, "outside/source.txt")
			write(t, target, []byte("must stay unchanged"), 0600)
			link := filepath.Join(f.root, outputDirectory, name)
			if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
				t.Fatal(err)
			}
			point := target
			if name == "audit" {
				point = filepath.Dir(target)
			}
			if err := os.Symlink(point, link); err != nil {
				t.Fatal(err)
			}
			if output, err := boundedCommand(t, f.command("audit-candidate.sh", outputDirectory)); err == nil {
				t.Fatalf("unsafe output accepted: %s", output)
			}
			data, err := os.ReadFile(target)
			if err != nil || string(data) != "must stay unchanged" {
				t.Fatalf("audit wrote through symlink: %v, %s", err, data)
			}
		})
	}
}
