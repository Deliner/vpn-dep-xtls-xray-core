// Package masqueprobe prepares source-verified diagnostic overlays, never releases.
package masqueprobe

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/platform/safeio"
)

// Overlay observes queues and inner TCP headers without changing admission,
// error handling or packet sizes. Every source is reviewed before writes.
func Overlay(dependency, source, output string) error {
	dependency, err := filepath.Abs(dependency)
	if err != nil {
		return err
	}
	changes, policy, err := inspectSources(dependency)
	if err != nil {
		return err
	}
	packets, err := inspectPackets(source)
	if err != nil {
		return err
	}
	changes = append(changes, packets...)
	device, err := inspectDevice(source)
	if err != nil {
		return err
	}
	changes = append(changes, device...)
	return publishOverlay(output, changes, policy)
}

func publishOverlay(output string, changes []sourceChange, policy string) error {
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
		"schema_version": 7, "kind": "diagnostic-only", "queue_policy": policy,
		"connection_queue_policy": connectionQueuePolicy(changes),
		"send_queue_policy":       sendQueuePolicy(changes),
		"default_queue_limit":     32, "default_connection_queue_limit": 128, "sources": observations,
	})
}
