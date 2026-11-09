package overlay

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

func Publish(output string, changes []Change, policies Policies) error {
	directory := filepath.Join(output, "overlay")
	if err := os.Mkdir(directory, 0700); err != nil {
		return err
	}
	replacements := map[string]string{}
	observations := map[string]any{}
	for _, change := range changes {
		replacement := filepath.Join(directory, filepath.Base(change.Target))
		if err := os.WriteFile(replacement, change.Modified, 0600); err != nil {
			return err
		}
		replacements[change.Target] = replacement
		observations[filepath.Base(change.Target)] = map[string]string{
			"target": change.Target, "source_sha256": change.SourceSHA,
			"instrumented_sha256": fmt.Sprintf("%x", sha256.Sum256(change.Modified)),
		}
	}
	if err := safeio.JSON(filepath.Join(output, "overlay.json"), map[string]any{"Replace": replacements}); err != nil {
		return err
	}
	return safeio.JSON(filepath.Join(output, "probe.json"), map[string]any{
		"schema_version": 9, "kind": "diagnostic-only", "queue_policy": policies.HTTP3,
		"connection_queue_policy": policies.Connection,
		"send_queue_policy":       policies.Send,
		"server_queue_policy":     "drop-new-when-full", "server_queue_limit": policies.ServerQueueLimit,
		"default_queue_limit": 32, "default_connection_queue_limit": 128, "sources": observations,
	})
}
