package diagnostics

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

type runtimeLog struct {
	Test   string `json:"test"`
	Label  string `json:"label"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

// RetainRuntimeLog preserves a closed process log regardless of test outcome.
// The caller must stop and wait for the process before calling. An empty output
// disables retention; otherwise failures must fail the enclosing experiment.
func RetainRuntimeLog(output, test, label, source string) error {
	if output == "" {
		return nil
	}
	raw, err := safeio.Read(source, 64<<20)
	if err != nil {
		return fmt.Errorf("read runtime log %s: %w", label, err)
	}
	if err := safeio.PrivateDir(output); err != nil {
		return err
	}
	directory, err := os.MkdirTemp(output, "runtime-")
	if err != nil {
		return err
	}
	if err := safeio.Atomic(filepath.Join(directory, "runtime.log"), raw, 0600); err != nil {
		return err
	}
	record := runtimeLog{Test: test, Label: label, Bytes: len(raw), SHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
	return safeio.JSON(filepath.Join(directory, "record.json"), record)
}
