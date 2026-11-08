// Package journal owns private rollout plans and immutable candidate bytes.
// It never publishes assignments or decides whether another stage may start.
package journal

import (
	"os"

	"vpn-script/internal/core/fleet/wire"
)

const MaxPlans = 10000

type Store struct {
	root string
	lock *os.File
}

type Target struct {
	Generation        string `json:"generation"`
	RequestID         string `json:"request_id"`
	PublishedRevision uint64 `json:"published_revision,string"`
	Phase             string `json:"phase"`
	LastSeen          int64  `json:"last_seen"`
	HealthySince      int64  `json:"healthy_since"`
	FirstReadyAttempt uint64 `json:"first_ready_attempt,string"`
}

type Record struct {
	Version        int              `json:"version"`
	FleetID        string           `json:"fleet_id"`
	Spec           wire.RolloutSpec `json:"spec"`
	SpecSHA256     string           `json:"spec_sha256"`
	Phase          string           `json:"phase"`
	Stage          int              `json:"stage"`
	CreatedAt      int64            `json:"created_at"`
	UpdatedAt      int64            `json:"updated_at"`
	StageStartedAt int64            `json:"stage_started_at"`
	Failure        string           `json:"failure,omitempty"`
	FailedNode     string           `json:"failed_node,omitempty"`
	Targets        [][]Target       `json:"targets"`
}
