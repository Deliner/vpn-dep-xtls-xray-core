package runtimeaudit

import (
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/diagnostics"
	"vpn-script/internal/runtime/engines"
)

type runtimeAudit struct {
	scanner   string
	directory string
	version   string
}

func (audit runtimeAudit) report(engine, arch string) string {
	return filepath.Join(audit.directory, engine+"-linux-"+arch+".txt")
}

func (audit runtimeAudit) failure(t *testing.T, engine, arch string, scanErr error) {
	t.Helper()
	contents, readErr := safeio.Read(audit.report(engine, arch), 2_000_000)
	if readErr == nil {
		t.Log(string(contents))
	}
	t.Errorf("runtime audit failed: %v; see %s\n%s", errors.Join(scanErr, readErr),
		audit.report(engine, arch), diagnostics.SecurityAudit.Instruction())
}

func runtimeAuditFromEnvironment(t *testing.T) *runtimeAudit {
	t.Helper()
	audit := &runtimeAudit{scanner: os.Getenv("RUNTIME_AUDIT_SCANNER"), directory: os.Getenv("RUNTIME_AUDIT_REPORT")}
	if audit.scanner == "" && audit.directory == "" {
		return nil
	}
	if !filepath.IsAbs(audit.scanner) || !filepath.IsAbs(audit.directory) {
		t.Fatal("runtime audit requires absolute scanner and report paths; use just audit-runtime")
	}
	require(t, safeio.PrivateDir(audit.directory))
	for _, engine := range []string{"xray", "sing-box"} {
		for _, arch := range []string{"x86_64", "aarch64"} {
			require(t, safeio.Atomic(audit.report(engine, arch), []byte("Scan not completed in this run.\n"), 0600))
		}
	}
	version, err := process.Run(context.Background(), 30*time.Second, audit.scanner, "-version")
	audit.version = "started_at=" + time.Now().UTC().Format(time.RFC3339) + "\n" + string(version.Output)
	require(t, safeio.Atomic(filepath.Join(audit.directory, "scanner.txt"), []byte(audit.version), 0600))
	require(t, err)
	return audit
}

func auditHeader(path, engine, arch string) (string, error) {
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash, err := safeio.FileHash(path)
	if err != nil {
		return "", err
	}
	runtime := engines.Runtime(engine)
	asset := runtime.Assets[arch]
	header := fmt.Sprintf("engine=%s\npinned_version=%s\narchitecture=%s\narchive_url=%s\narchive_sha256=%s\nbinary_sha256=%s\n",
		engine, runtime.Version, arch, asset.URL, asset.SHA256, hash)
	if info.Main.Version == "" || info.Main.Version == "(devel)" {
		header += "coverage_limit=main module version is unknown; engine advisories need separate review\n"
	}
	header += "coverage_limit=Go binary symbols/dependencies only; no call graph or C/libcronet audit\n\n" + info.String() + "\n"
	return header, nil
}

func (audit runtimeAudit) scan(path, engine, arch string) error {
	header, err := auditHeader(path, engine, arch)
	if err != nil {
		return err
	}
	output, err := os.CreateTemp(audit.directory, ".scan-")
	if err != nil {
		return err
	}
	defer os.Remove(output.Name())
	if _, err := fmt.Fprint(output, audit.version, "\n", header); err != nil {
		return errors.Join(err, output.Close())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := process.Command(ctx, audit.scanner, "-mode", "binary", "-show", "verbose", path)
	command.Stdout, command.Stderr = output, output
	defer process.Stop(command)
	scanErr := command.Run()
	upstreamErr := auditLocalReplacements(ctx, audit.scanner, path, output)
	_, writeErr := fmt.Fprintf(output, "\nscan_error=%v\nupstream_audit_error=%v\n", scanErr, upstreamErr)
	closeErr := output.Close()
	renameErr := os.Rename(output.Name(), audit.report(engine, arch))
	return errors.Join(scanErr, upstreamErr, writeErr, closeErr, renameErr)
}
