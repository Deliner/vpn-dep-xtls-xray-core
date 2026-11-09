package integration

import (
	"net"
	"net/http/httptest"
	"sync/atomic"

	"vpn-script/internal/control/integration/evidence"
	"vpn-script/internal/control/integration/evidence/installation"
	"vpn-script/internal/control/integration/selection"
	"vpn-script/internal/control/integration/traffic"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
)

type origins struct {
	http     *httptest.Server
	udp      *net.UDPConn
	httpHits *atomic.Int64
	udpHits  *atomic.Int64
}

type lab struct {
	runtimes  selection.Set
	directory string
	topology  *model.Topology
	secrets   model.Secrets
	origin    origins
	ports     []int
	nodeIPs   []string
	servers   map[string]func()
	clients   map[string]int
}

type concurrentSuite struct {
	Platform      string                      `json:"platform"`
	TestGoVersion string                      `json:"test_go_version"`
	SchemaVersion int                         `json:"schema_version"`
	Status        string                      `json:"status"`
	Scope         string                      `json:"scope"`
	Hops          int                         `json:"hops"`
	NodeAddresses []string                    `json:"node_addresses"`
	Services      int                         `json:"services"`
	Profiles      []string                    `json:"profiles"`
	Paths         map[string][]string         `json:"paths"`
	StoppedClient string                      `json:"stopped_client,omitempty"`
	Runtimes      map[string]evidence.Runtime `json:"runtimes"`
	Assignments   selection.Set               `json:"runtime_assignments"`
	Phases        []traffic.Report            `json:"phases"`
	StartedAt     string                      `json:"started_at"`
	FinishedAt    string                      `json:"finished_at"`
}

type installedSuite struct {
	SchemaVersion            int                          `json:"schema_version"`
	Status                   string                       `json:"status"`
	Profiles                 []string                     `json:"profiles"`
	Generations              []string                     `json:"generations"`
	Phases                   []traffic.Report             `json:"phases"`
	Runtimes                 map[string]evidence.Runtime  `json:"runtimes"`
	UnchangedPID             bool                         `json:"unchanged_pid"`
	RolledBack               bool                         `json:"rolled_back"`
	Recovered                bool                         `json:"stopped_service_recovered"`
	PortsReleased            bool                         `json:"old_ports_released"`
	CleanStop                bool                         `json:"clean_stop"`
	ReleaseIDs               []string                     `json:"release_ids"`
	Upgraded                 bool                         `json:"installer_upgraded"`
	UpgradeRollback          bool                         `json:"installer_upgrade_rolled_back"`
	Migrated                 bool                         `json:"legacy_state_migrated"`
	UnitRepaired             bool                         `json:"unit_repaired"`
	PreviousSHA              string                       `json:"previous_installer_sha256,omitempty"`
	InstallerSHA             string                       `json:"installer_sha256,omitempty"`
	Interruption             *installation.Report         `json:"installer_interruption,omitempty"`
	CachedArchives           map[string]string            `json:"cached_archives_sha256"`
	CachedBytes              int64                        `json:"cached_archive_bytes"`
	CacheMissBlocked         bool                         `json:"cache_miss_blocked"`
	CacheOffline             bool                         `json:"cache_offline_lifecycle"`
	CacheProbeRequests       int64                        `json:"cache_probe_requests"`
	CacheWarmRequests        int64                        `json:"cache_warm_requests"`
	AccessProtected          bool                         `json:"installed_access_protected"`
	AccessChecks             []string                     `json:"installed_access_checks"`
	SharedPorts              []int                        `json:"shared_tcp_udp_listener_ports"`
	UDPWithSSH               bool                         `json:"udp22_with_existing_tcp_ssh"`
	IdentityChecks           []string                     `json:"service_identity_checks"`
	IdentityProtected        bool                         `json:"service_identity_protected"`
	ProcessAuthority         map[string]map[string]string `json:"process_authority"`
	WildcardListeners        []string                     `json:"wildcard_listener_tags"`
	DisjointForeignSocket    string                       `json:"disjoint_foreign_tcp_socket"`
	DisjointForeignPreserved bool                         `json:"disjoint_foreign_tcp_preserved"`
}

var installedFailures = map[string]string{
	"failed": "service failed process/listener checks", "start-failed": "systemctl failed",
	"download-failed": "runtime HTTPS request failed", "access-failed": "unsafe managed access",
	"contents-failed": "committed release contents differ",
	"identity-failed": "unsafe service identity",
}

type installedRevision struct {
	State     installer.State
	Directory string
	Unit      []byte
}
