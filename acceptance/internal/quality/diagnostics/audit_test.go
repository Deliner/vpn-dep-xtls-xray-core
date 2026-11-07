package diagnostics

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditRetainsEarlyFindingsAndScanFailures(t *testing.T) {
	for _, status := range []int{0, 3, 7} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			scanner := auditScanner(t, status, 0)
			reports := t.TempDir()
			command := exec.Command("bash", "../../../ops/quality/audit.sh", scanner, reports)
			output, err := command.CombinedOutput()
			want := 0
			if status != 0 {
				want = 1
			}
			checkAuditExit(t, err, want, output)
			for _, target := range []string{"linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"} {
				contents, err := os.ReadFile(filepath.Join(reports, target+".txt"))
				if err != nil || !strings.Contains(string(contents), "scanned "+target) {
					t.Fatalf("audit did not finish %s after an earlier result: %v, %s", target, err, contents)
				}
			}
		})
	}
}

func TestAuditVersionFailureInvalidatesPreviousSuccess(t *testing.T) {
	reports := t.TempDir()
	old := filepath.Join(reports, "linux-amd64.txt")
	if err := os.WriteFile(old, []byte("No vulnerabilities found."), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", "../../../ops/quality/audit.sh", auditScanner(t, 0, 7), reports)
	output, err := command.CombinedOutput()
	checkAuditExit(t, err, 7, output)
	if !strings.Contains(string(output), "scanner unavailable") {
		t.Fatalf("lost scanner diagnostic: %s", output)
	}
	contents, err := os.ReadFile(old)
	if err != nil || strings.Contains(string(contents), "No vulnerabilities found.") {
		t.Fatalf("previous report still looks successful: %v, %s", err, contents)
	}
}

func checkAuditExit(t *testing.T, err error, want int, output []byte) {
	t.Helper()
	if want == 0 && err == nil {
		return
	}
	var failure *exec.ExitError
	if !errors.As(err, &failure) || failure.ExitCode() != want {
		t.Fatalf("want exit %d, got %v: %s", want, err, output)
	}
}

func auditScanner(t *testing.T, status, versionStatus int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture scanner")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = '-version' ]; then
  echo 'scanner unavailable or fixture version'
  exit %d
fi
[ "$*" = '-test -show traces,verbose ./...' ] || exit 9
[ "$CGO_ENABLED" = 0 ] || exit 9
echo "scanned $GOOS-$GOARCH"
if [ "$GOOS-$GOARCH" = linux-amd64 ]; then exit %d; fi
`, versionStatus, status)
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
