package runtimebuild

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func auditFixture(t *testing.T, engine string) fixture {
	t.Helper()
	f := newFixture(t)
	write(t, filepath.Join(f.root, outputDirectory, "status"), []byte("built\n"), 0600)
	metadata := "engine=" + engine + "\ntarget=linux/amd64\nbuild_cgo=0\nbuild_tags=with_gvisor,with_naive_outbound,with_purego\n"
	write(t, filepath.Join(f.root, outputDirectory, "provenance.txt"), []byte(metadata), 0600)
	var hashes strings.Builder
	paths := []string{"source/go.mod"}
	if engine == "sing-box" {
		paths = append(paths, "bin/libcronet.so")
	}
	for _, path := range append(paths, "bin/"+engine) {
		data := []byte("fixture contents\n")
		write(t, filepath.Join(f.root, outputDirectory, path), data, 0600)
		fmt.Fprintf(&hashes, "%x  %s\n", sha256.Sum256(data), path)
	}
	write(t, filepath.Join(f.root, outputDirectory, "SHA256SUMS"), []byte(hashes.String()), 0600)
	write(t, filepath.Join(f.root, "build/tools/bin/govulncheck"), []byte(fakeScanner), 0700)
	return f
}

func TestCandidateAuditPreservesFailureAndCompletesBothScans(t *testing.T) {
	for _, statuses := range [][2]int{{3, 0}, {0, 7}} {
		t.Run(fmt.Sprint(statuses), func(t *testing.T) {
			f := auditFixture(t, "xray")
			command := f.command("audit-candidate.sh", outputDirectory)
			command.Env = append(command.Env, fmt.Sprintf("SOURCE_STATUS=%d", statuses[0]), fmt.Sprintf("BINARY_STATUS=%d", statuses[1]))
			if output, err := command.CombinedOutput(); err == nil {
				t.Fatalf("scanner failure hidden: %s", output)
			}
			for i, scan := range []string{"source", "binary"} {
				data, err := os.ReadFile(filepath.Join(f.root, outputDirectory, "audit", scan+".txt"))
				if err != nil || !strings.Contains(string(data), fmt.Sprintf("scanner_exit_status=%d", statuses[i])) {
					t.Fatalf("lost %s scan result: %v, %s", scan, err, data)
				}
			}
		})
	}
}

func TestCandidateAuditRejectsChangedInputAndInvalidatesOldReports(t *testing.T) {
	for _, path := range []string{"source/go.mod", "source/added.go", "bin/xray"} {
		t.Run(path, func(t *testing.T) {
			f := auditFixture(t, "xray")
			write(t, filepath.Join(f.root, outputDirectory, path), []byte("changed"), 0600)
			for _, scan := range []string{"source", "binary"} {
				write(t, filepath.Join(f.root, outputDirectory, "audit", scan+".txt"), []byte("old success"), 0600)
			}
			if output, err := f.command("audit-candidate.sh", outputDirectory).CombinedOutput(); err == nil {
				t.Fatalf("changed input accepted: %s", output)
			}
			for _, scan := range []string{"source", "binary"} {
				data, err := os.ReadFile(filepath.Join(f.root, outputDirectory, "audit", scan+".txt"))
				if err != nil || !strings.Contains(string(data), "not completed") {
					t.Fatalf("stale successful audit remains: %v, %s", err, data)
				}
			}
		})
	}
}

const fakeScanner = `#!/usr/bin/env bash
printf 'CGO_ENABLED=%s %s\n' "$CGO_ENABLED" "$*" >> "$FIXTURE_TRACE"
case "$1" in
  -version) printf 'govulncheck fixture\n' ;;
  -C) printf 'source finding or failure\n'; exit "${SOURCE_STATUS:-0}" ;;
  -mode) printf 'binary finding or failure\n'; exit "${BINARY_STATUS:-0}" ;;
  *) exit 2 ;;
esac
`
