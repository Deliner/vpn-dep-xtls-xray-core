// Package masqueprobe prepares source-verified diagnostic overlays, never releases.
package masqueprobe

import (
	"path/filepath"

	"vpn-script/internal/quality/runtimebuild/masqueprobe/overlay"
	"vpn-script/internal/quality/runtimebuild/masqueprobe/packets"
)

// Overlay observes queues and inner packet headers without changing admission,
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
	connectIP, err := packets.InspectConnectIP(source)
	if err != nil {
		return err
	}
	changes = append(changes, connectIP...)
	device, err := packets.InspectDevice(source)
	if err != nil {
		return err
	}
	changes = append(changes, device...)
	server, err := packets.InspectServer(source)
	if err != nil {
		return err
	}
	changes = append(changes, server...)
	return overlay.Publish(output, changes, overlay.Policies{
		HTTP3: policy, Connection: connectionQueuePolicy(changes), Send: sendQueuePolicy(changes),
		ServerQueueLimit: packets.ServerQueueLimit,
	})
}
