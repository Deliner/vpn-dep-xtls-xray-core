package runtimeaudit

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinaryAuditPreservesFindingsAndScannerFailures(t *testing.T) {
	binary, err := os.Executable()
	require(t, err)
	for _, status := range []int{0, 3, 7} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			directory := t.TempDir()
			scanner := filepath.Join(directory, "fixture scanner")
			script := fmt.Sprintf("#!/bin/sh\n[ \"$1 $2\" = '-mode binary' ] || exit 9\necho 'original scanner diagnostic'\nexit %d\n", status)
			require(t, os.WriteFile(scanner, []byte(script), 0700))
			audit := runtimeAudit{scanner: scanner, directory: directory}
			err := audit.scan(binary, "xray", "x86_64")
			if status == 0 && err != nil {
				t.Fatal(err)
			}
			var failure *exec.ExitError
			if status != 0 && (!errors.As(err, &failure) || failure.ExitCode() != status) {
				t.Fatalf("lost scanner failure %d: %v", status, err)
			}
			report, err := os.ReadFile(audit.report("xray", "x86_64"))
			require(t, err)
			for _, expected := range []string{"original scanner diagnostic", "binary_sha256=", "coverage_limit=", "scan_error="} {
				if !strings.Contains(string(report), expected) {
					t.Fatalf("missing %s in runtime audit report", expected)
				}
			}
		})
	}
}
