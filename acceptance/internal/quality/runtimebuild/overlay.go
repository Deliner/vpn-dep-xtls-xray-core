package runtimebuild

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

const masqueQueueSource = "fb9d3debf96571f2a468a9bbccdeb6112659dcde63f990e5d6a7236cfc44b1df"

// MasqueOverlay instruments one exact dependency file without changing the module
// cache or candidate source. The probe observes queue drops; its bound stays 32.
func MasqueOverlay(source, output string) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	raw, err := safeio.Read(source, 1024*1024)
	if err != nil {
		return err
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != masqueQueueSource {
		return fmt.Errorf("unreviewed HTTP/3 queue source; diagnostic overlay refused")
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
		"kind": "diagnostic-only", "target": source, "target_sha256": masqueQueueSource,
		"instrumented_sha256": fmt.Sprintf("%x", sha256.Sum256(modified)), "queue_limit": 32,
	})
}
