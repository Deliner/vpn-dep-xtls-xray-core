package wire

// Candidates stage verified manifests without changing any node assignment.
type CandidateUpload struct {
	Manifest []byte `json:"manifest"`
}

type Candidate struct {
	NodeID     string `json:"node_id"`
	SHA256     string `json:"sha256"`
	Generation string `json:"generation"`
}

type RolloutTarget struct {
	NodeID           string `json:"node_id"`
	CandidateSHA256  string `json:"candidate_sha256"`
	ExpectedRevision uint64 `json:"expected_revision,string"`
}

type RolloutStage struct {
	Name    string          `json:"name"`
	Targets []RolloutTarget `json:"targets"`
}

// Stages are explicit ordered groups. The first is the canary; the remaining
// groups are never published until the preceding groups pass their health gate.
type RolloutSpec struct {
	Version             int            `json:"version"`
	ID                  string         `json:"id"`
	Stages              []RolloutStage `json:"stages"`
	ObserveSeconds      int64          `json:"observe_seconds"`
	FreshSeconds        int64          `json:"fresh_seconds"`
	StageTimeoutSeconds int64          `json:"stage_timeout_seconds"`
	ValiditySeconds     int64          `json:"validity_seconds"`
}

type RolloutNode struct {
	NodeID            string `json:"node_id"`
	Stage             int    `json:"stage"`
	Generation        string `json:"generation"`
	PublishedRevision uint64 `json:"published_revision,string"`
	Phase             string `json:"phase"`
	LastSeen          int64  `json:"last_seen"`
	HealthySince      int64  `json:"healthy_since"`
}

type Rollout struct {
	ID             string        `json:"id"`
	FleetID        string        `json:"fleet_id"`
	SpecSHA256     string        `json:"spec_sha256"`
	Phase          string        `json:"phase"`
	Stage          int           `json:"stage"`
	StageName      string        `json:"stage_name"`
	CreatedAt      int64         `json:"created_at"`
	UpdatedAt      int64         `json:"updated_at"`
	StageStartedAt int64         `json:"stage_started_at"`
	Failure        string        `json:"failure,omitempty"`
	FailedNode     string        `json:"failed_node,omitempty"`
	TotalNodes     int           `json:"total_nodes"`
	PublishedNodes int           `json:"published_nodes"`
	ReadyNodes     int           `json:"ready_nodes"`
	Nodes          []RolloutNode `json:"nodes,omitempty"`
}
