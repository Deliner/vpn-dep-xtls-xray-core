package session

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestSnapshotRejectsChangedRuntimeAndPinsCopiedBytes(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "input")
	original := []byte("reviewed runtime bytes")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	artifact := Artifact{Path: path, SHA256: safeio.Hash(original)}
	if err := copyArtifact(directory, "native", artifact); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replaced after review"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := copyArtifact(directory, "tampered", artifact); err == nil {
		t.Fatal("runtime changed after pinning was accepted")
	}
	if _, err := os.Stat(filepath.Join(directory, "tampered")); !os.IsNotExist(err) {
		t.Fatal("unverified executable was published")
	}
	data, err := os.ReadFile(filepath.Join(directory, "native"))
	if err != nil || string(data) != string(original) {
		t.Fatal("source replacement changed the private execution snapshot")
	}
}

func TestActivationRefusesNonLinuxWithoutChangingHost(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("non-Linux refusal contract")
	}
	path := filepath.Join(t.TempDir(), "session")
	if s, err := Start(t.Context(), Request{Directory: path}); s != nil || err == nil {
		t.Fatal("activation was allowed outside Linux")
	}
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatal("platform refusal modified the filesystem")
	}
}

func TestReconnectRequiresExactConfigurationBytesAndPermissions(t *testing.T) {
	directory, value := t.TempDir(), map[string]any{"tag": "private", "password": "<secret>"}
	path := filepath.Join(directory, "frontend.json")
	if err := safeio.NewJSON(path, value); err != nil {
		t.Fatal(err)
	}
	if err := verifyConfiguration(directory, "frontend.json", value, 0600, os.Getegid()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyConfiguration(directory, "frontend.json", value, 0600, os.Getegid()); err == nil {
		t.Fatal("changed snapshot bytes were accepted")
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if err := verifyConfiguration(directory, "frontend.json", value, 0600, os.Getegid()); err == nil {
		t.Fatal("world-readable snapshot was accepted")
	}
}
