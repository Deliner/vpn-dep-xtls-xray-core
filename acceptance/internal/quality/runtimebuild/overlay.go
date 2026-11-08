package runtimebuild

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

// MasqueOverlay instruments one exact dependency file without changing the module
// cache or candidate source. The probe observes drops without changing queue policy.
func MasqueOverlay(source, output string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	raw, err := safeio.Read(source, 1024*1024)
	if err != nil {
		return err
	}
	policy, sourceSHA, err := masqueQueuePolicy(raw)
	if err != nil {
		return err
	}
	modified, err := instrumentMasqueQueue(raw)
	if err != nil {
		return err
	}
	directory := filepath.Join(output, "overlay")
	if err := os.Mkdir(directory, 0700); err != nil {
		return err
	}
	replacement := filepath.Join(directory, "state_tracking_stream.go")
	if err := os.WriteFile(replacement, modified, 0600); err != nil {
		return err
	}
	if err := safeio.JSON(filepath.Join(output, "overlay.json"), map[string]any{"Replace": map[string]string{source: replacement}}); err != nil {
		return err
	}
	return safeio.JSON(filepath.Join(output, "probe.json"), map[string]any{
		"kind": "diagnostic-only", "target": source, "target_sha256": sourceSHA,
		"instrumented_sha256": fmt.Sprintf("%x", sha256.Sum256(modified)), "default_queue_limit": 32,
		"queue_policy": policy,
	})
}
