// Package node verifies installer evidence without invoking the installer or engines.
package node

type report struct {
	Schema       int                      `json:"schema_version"`
	Status       string                   `json:"status"`
	Profiles     []string                 `json:"profiles"`
	Phases       []phase                  `json:"phases"`
	Runtimes     map[string]runtimeRecord `json:"runtimes"`
	PreviousSHA  string                   `json:"previous_installer_sha256"`
	InstallerSHA string                   `json:"installer_sha256"`
	Archives     map[string]string        `json:"cached_archives_sha256"`
	CachedBytes  int64                    `json:"cached_archive_bytes"`
	ColdRequests int                      `json:"cache_probe_requests"`
	WarmRequests int                      `json:"cache_warm_requests"`
	Interruption crash                    `json:"installer_interruption"`
}

type runtimeRecord struct {
	SHA      string `json:"sha256"`
	Verified bool   `json:"verified_unchanged"`
	Error    string `json:"error"`
}

type phase struct {
	Name     string            `json:"phase"`
	Profiles int               `json:"profiles"`
	Required int               `json:"required_rounds"`
	Barriers int               `json:"barrier_rounds"`
	Streams  int               `json:"concurrent_tcp_udp_streams"`
	TCPBytes int               `json:"tcp_bytes_per_round"`
	UDPBytes int               `json:"udp_bytes_per_round"`
	Missing  []string          `json:"missing_arrivals"`
	Routes   map[string]rounds `json:"routes"`
}

type rounds struct {
	TCP   int    `json:"tcp_rounds"`
	UDP   int    `json:"udp_rounds"`
	Error string `json:"error"`
}

type crash struct {
	InstallerPID int    `json:"installer_pid"`
	ControlPID   int    `json:"systemd_control_pid"`
	Signal       string `json:"signal"`
	SubState     string `json:"systemd_substate"`
	Previous     string `json:"previous_release"`
	Next         string `json:"interrupted_release"`
	JournalSHA   string `json:"journal_sha256"`
	Restored     bool   `json:"previous_state_restored"`
}

type transaction struct {
	Schema int      `json:"schema_version"`
	Phase  string   `json:"phase"`
	Old    snapshot `json:"old"`
	Next   snapshot `json:"next"`
}

type snapshot struct {
	Release string `json:"release"`
	State   []byte `json:"state"`
	Active  bool   `json:"active"`
	Enabled bool   `json:"enabled"`
}
