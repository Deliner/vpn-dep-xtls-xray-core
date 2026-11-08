package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/node/fleet/service"
	"vpn-script/internal/platform/fleetapi"
	"vpn-script/internal/platform/safeio"
)

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func privateDirectory(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	require(t, err)
	require(t, os.Chmod(path, 0700))
	return path
}

func TestFleetAgentRefusesMacBeforeAccessingFiles(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS host protection contract")
	}
	for _, args := range [][]string{{"agent-init", "--state", "missing", "--bootstrap", "missing"}, {"agent-run", "--state", "missing"},
		{"agent-service-install", "--state", "missing"}, {"agent-service-start"}, {"agent-service-stop"}, {"agent-service-remove"}} {
		var output bytes.Buffer
		err := Run(context.Background(), args, &output)
		if err == nil || !strings.Contains(err.Error(), "Linux root") || output.Len() != 0 {
			t.Fatalf("Mac guard did not precede file access: %v", err)
		}
	}
}

func TestFleetCredentialRefusalUsesNonRestartableExitStatus(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		err := errors.Join(errors.New("local context"), &fleetapi.HTTPError{Status: status})
		if ExitCode(err) != service.AccessDeniedExit {
			t.Fatal("credential refusal would be restarted by systemd")
		}
	}
	if ExitCode(nil) != 0 || ExitCode(&fleetapi.HTTPError{Status: http.StatusServiceUnavailable}) != 1 {
		t.Fatal("ordinary completion or retryable failure lost its exit status")
	}
}

func TestControllerInitializationIsPrivateAndNeverOverwrites(t *testing.T) {
	root := filepath.Join(privateDirectory(t), "controller")
	args := []string{"init", "--state", root, "--fleet", "test-fleet"}
	var output bytes.Buffer
	require(t, Run(context.Background(), args, &output))
	before, err := os.ReadFile(filepath.Join(root, "authority.json"))
	require(t, err)
	if bytes.Contains(output.Bytes(), []byte("seed")) || bytes.Contains(output.Bytes(), []byte("token")) {
		t.Fatal("initialization printed private key material")
	}
	if err := Run(context.Background(), args, &output); err == nil {
		t.Fatal("existing authority overwritten")
	}
	after, err := os.ReadFile(filepath.Join(root, "authority.json"))
	require(t, err)
	if !bytes.Equal(before, after) {
		t.Fatal("failed initialization changed authority")
	}
	require(t, safeio.CheckAccess(root, os.ModeDir|0700, os.Geteuid(), -1))
	require(t, safeio.CheckAccess(filepath.Join(root, "admin-credential.json"), 0600, os.Geteuid(), -1))
}

func TestPrivatePublicationReaderSupportsFullManifestSize(t *testing.T) {
	path := filepath.Join(privateDirectory(t), "request.json")
	request := wire.Publish{RequestID: strings.Repeat("a", 64), Manifest: bytes.Repeat([]byte("x"), contract.MaxManifestSize)}
	require(t, safeio.NewJSON(path, request))
	var restored wire.Publish
	require(t, privateRead(path, &restored))
	if !bytes.Equal(restored.Manifest, request.Manifest) {
		t.Fatal("maximum-size manifest truncated by private request reader")
	}
	require(t, os.Chmod(path, 0644))
	if err := privateRead(path, &restored); err == nil {
		t.Fatal("public request file accepted")
	}
}

func TestPrivatePathsRejectReplaceableAndSymlinkParents(t *testing.T) {
	root := privateDirectory(t)
	parent := filepath.Join(root, "private")
	require(t, os.Mkdir(parent, 0700))
	link := filepath.Join(root, "link")
	require(t, os.Symlink(parent, link))
	if err := newOutput(filepath.Join(link, "output")); err == nil {
		t.Fatal("symlink parent accepted")
	}
	require(t, os.Chmod(parent, 0777))
	if err := newOutput(filepath.Join(parent, "output")); err == nil {
		t.Fatal("replaceable parent accepted")
	}
}
