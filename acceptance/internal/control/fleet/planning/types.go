// Package planning prepares a reviewable rollout request from local manifests.
// It stages immutable candidates, never publishes desired assignments.
package planning

import (
	"context"

	"vpn-script/internal/core/fleet/wire"
)

type Remote interface {
	Node(context.Context, string, string) (wire.Node, error)
	StageCandidate(context.Context, string, string, []byte) (wire.Candidate, error)
}

type draft struct {
	Version             int     `yaml:"version"`
	ID                  string  `yaml:"id"`
	ObserveSeconds      int64   `yaml:"observe_seconds"`
	FreshSeconds        int64   `yaml:"fresh_seconds"`
	StageTimeoutSeconds int64   `yaml:"stage_timeout_seconds"`
	ValiditySeconds     int64   `yaml:"validity_seconds"`
	Stages              []stage `yaml:"stages"`
}

type stage struct {
	Name      string   `yaml:"name"`
	Manifests []string `yaml:"manifests"`
}

type input struct {
	path string
	meta wire.Candidate
}
