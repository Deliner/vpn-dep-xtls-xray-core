package installer

import (
	"context"
	"time"

	"vpn-script/internal/node/journal"
	"vpn-script/internal/platform/process"
)

type Paths struct {
	Base     string
	Unit     string
	Firewall string
}

type Installer struct {
	Paths   Paths
	Run     process.Runner
	Sleep   func(time.Duration)
	Context context.Context
}

type State struct {
	Files              map[string]string `json:"files,omitempty"`
	TopologyID         string            `json:"topology_id"`
	NodeID             string            `json:"node_id"`
	Generation         string            `json:"generation"`
	ReleaseID          string            `json:"release_id,omitempty"`
	PreviousGeneration string            `json:"previous_generation,omitempty"`
	PreviousReleaseID  string            `json:"previous_release_id,omitempty"`
	ConfigSHA256       string            `json:"config_sha256"`
	BinarySHA256       string            `json:"binary_sha256"`
	Firewall           string            `json:"firewall"`
	Verification       string            `json:"verification"`
}

// Older state files used the configuration generation as the directory name.
func (s *State) releaseID() string {
	if s.ReleaseID != "" {
		return s.ReleaseID
	}
	return s.Generation
}

type preparedRelease struct {
	path    string
	created bool
}

type Snapshot = journal.Snapshot

type Result struct {
	Node         string `json:"node"`
	Generation   string `json:"generation"`
	Result       string `json:"result"`
	Verification string `json:"verification"`
}
