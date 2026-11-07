package evidence

import "time"

// report is the independently checked subset of the versioned payload report contract.
type report struct {
	SchemaVersion int                 `json:"schema_version"`
	Status        string              `json:"status"`
	Platform      string              `json:"platform"`
	TestGoVersion string              `json:"test_go_version"`
	Hops          int                 `json:"hops"`
	NodeAddresses []string            `json:"node_addresses"`
	Services      int                 `json:"services"`
	Profiles      []string            `json:"profiles"`
	Paths         map[string][]string `json:"paths"`
	Stopped       string              `json:"stopped_client"`
	StartedAt     time.Time           `json:"started_at"`
	FinishedAt    time.Time           `json:"finished_at"`
	Runtimes      map[string]Runtime  `json:"runtimes"`
	Assignments   assignments         `json:"runtime_assignments"`
	Phases        []phase             `json:"phases"`
}

type assignments struct {
	Clients map[string]string            `json:"clients"`
	Nodes   map[string]map[string]string `json:"nodes"`
}

type phase struct {
	Name     string                 `json:"phase"`
	Profiles int                    `json:"profiles"`
	Streams  int                    `json:"concurrent_tcp_udp_streams"`
	Required int                    `json:"required_rounds"`
	Barriers int                    `json:"barrier_rounds"`
	TCPBytes int                    `json:"tcp_bytes_per_round"`
	UDPBytes int                    `json:"udp_bytes_per_round"`
	Missing  []string               `json:"missing_arrivals"`
	Routes   map[string]routeResult `json:"routes"`
}

type routeResult struct {
	TCPRounds int    `json:"tcp_rounds"`
	UDPRounds int    `json:"udp_rounds"`
	Error     string `json:"error"`
}
