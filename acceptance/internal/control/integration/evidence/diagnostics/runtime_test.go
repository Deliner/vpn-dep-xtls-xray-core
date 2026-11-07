package diagnostics

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimeLogsKeepRepeatedProcessesAndFinalBytes(t *testing.T) {
	directory := t.TempDir()
	source, output := filepath.Join(directory, "source.log"), filepath.Join(directory, "logs")
	for _, suffix := range []string{"first", "second"} {
		contents := "queue-drop\nfinal-" + suffix + "\n"
		if err := os.WriteFile(source, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
		if err := RetainRuntimeLog(output, "Suite/pass", "../same-label", source); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(output)
	if err != nil || len(entries) != 2 {
		t.Fatalf("lost repeated processes: %v, %v", entries, err)
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		seen[inspectLog(t, filepath.Join(output, entry.Name()))] = true
	}
	if !seen["queue-drop\nfinal-first\n"] || !seen["queue-drop\nfinal-second\n"] {
		t.Fatalf("lost final process output: %v", seen)
	}
}

func inspectLog(t *testing.T, directory string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(directory, "runtime.log"))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile(filepath.Join(directory, "record.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record runtimeLog
	if err := json.Unmarshal(metadata, &record); err != nil {
		t.Fatal(err)
	}
	if record.Test != "Suite/pass" || record.Label != "../same-label" || record.Bytes != len(raw) || record.SHA256 != fmt.Sprintf("%x", sha256.Sum256(raw)) {
		t.Fatalf("invalid log identity: %+v", record)
	}
	for _, item := range []string{directory, filepath.Join(directory, "runtime.log"), filepath.Join(directory, "record.json")} {
		info, err := os.Stat(item)
		if err != nil || info.Mode().Perm()&0077 != 0 {
			t.Fatalf("non-private log artifact: %s: %v", item, err)
		}
	}
	return string(raw)
}

func TestRuntimeLogFailureCannotLookLikeSuccessfulCapture(t *testing.T) {
	directory := t.TempDir()
	missing := filepath.Join(directory, "missing")
	if err := RetainRuntimeLog("", "test", "label", missing); err != nil {
		t.Fatal(err)
	}
	if err := RetainRuntimeLog(directory, "test", "label", missing); err == nil {
		t.Fatal("accepted missing runtime log")
	}
	if err := RetainRuntimeLog(directory, "test", "label", directory); err == nil {
		t.Fatal("accepted a directory as process output")
	}
	file := filepath.Join(directory, "too-large")
	if err := os.WriteFile(file, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(file, (64<<20)+1); err != nil {
		t.Fatal(err)
	}
	if err := RetainRuntimeLog(directory, "test", "label", file); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatalf("accepted incomplete bounded log: %v", err)
	}
}
