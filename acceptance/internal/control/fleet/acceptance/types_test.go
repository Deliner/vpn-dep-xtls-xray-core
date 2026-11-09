package acceptance

import (
	"crypto/tls"
	"net/http/httptest"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/control/integration/traffic"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/fleetapi"
)

const nodeID = "fleet-node"
const agentState = "/var/lib/nhop-fleet-agent"
const nodeBase = "/opt/vpn-nhop"
const nodeUnit = "/etc/systemd/system/vpn-nhop.service"

type controller struct {
	root        string
	store       *registry.Store
	server      *httptest.Server
	certificate tls.Certificate
	address     string
	client      *fleetapi.Client
	admin       string
}

type fixture struct {
	root       string
	binary     string
	controller *controller
	topology   *model.Topology
	secrets    model.Secrets
	updated    model.NodeConfig
	report     report
}

type report struct {
	Version              int                       `json:"version"`
	Status               string                    `json:"status"`
	Platform             string                    `json:"platform"`
	AgentSHA256          string                    `json:"agent_sha256"`
	SupervisorSHA256     string                    `json:"supervisor_sha256"`
	RuntimeSHA256        map[string]string         `json:"runtime_sha256"`
	Generations          []string                  `json:"generations"`
	Traffic              map[string]traffic.Report `json:"traffic"`
	Rollback             bool                      `json:"rollback"`
	AgentStopRetainsVPN  bool                      `json:"agent_stop_retains_vpn"`
	ControllerRecovery   bool                      `json:"controller_recovery"`
	AgentCrashRecovery   bool                      `json:"agent_crash_recovery"`
	CredentialsUnchanged bool                      `json:"credentials_unchanged"`
	RevokeExitStatus     string                    `json:"revoke_exit_status"`
	RevokeNoRestart      bool                      `json:"revoke_no_restart"`
	RemovedStateRetained bool                      `json:"removed_state_retained"`
	Interruption         *interruptionReport       `json:"installer_interruption,omitempty"`
}

type interruptionReport struct {
	AgentBefore       string `json:"agent_pid_before"`
	AgentAfter        string `json:"agent_pid_after"`
	ControlPID        int    `json:"systemd_control_pid"`
	Signal            string `json:"signal"`
	ExitStatus        string `json:"exit_status"`
	UnitResult        string `json:"unit_result"`
	PreviousRelease   string `json:"previous_release"`
	NextRelease       string `json:"next_release"`
	JournalSHA256     string `json:"journal_sha256"`
	IntentBeforeSHA   string `json:"intent_before_sha256"`
	IntentAfterSHA    string `json:"intent_after_sha256"`
	CredentialsBefore string `json:"credentials_before_sha256"`
	CredentialsAfter  string `json:"credentials_after_sha256"`
	Revision          uint64 `json:"revision,string"`
	PayloadSHA256     string `json:"payload_sha256"`
	AttemptBefore     uint64 `json:"attempt_before,string"`
	AttemptAfter      uint64 `json:"attempt_after,string"`
	Recovered         bool   `json:"recovered"`
}
