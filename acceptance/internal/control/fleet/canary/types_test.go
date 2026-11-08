package canary

import (
	"testing"
	"time"

	"vpn-script/internal/control/integration/traffic"
	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/model"
)

var nodes = []string{"fleet-c", "fleet-b", "fleet-a"} // Exit, relay, entry.

const controllerUnit = "nhop-canary-controller.service"
const agentState = "/var/lib/nhop-fleet-agent"
const nodeBase = "/opt/vpn-nhop"

type fixture struct {
	root, repository, lima, remote string
	addresses                      map[string]string
	topology                       *model.Topology
	secrets                        model.Secrets
	current                        map[string]model.NodeConfig
	report                         evidence
}

type machine struct {
	Address string `json:"address"`
	BootID  string `json:"boot_id"`
	Kernel  string `json:"kernel"`
}

type evidence struct {
	Status            string                    `json:"status"`
	Machines          map[string]machine        `json:"machines"`
	Rollouts          map[string]wire.Rollout   `json:"rollouts"`
	Traffic           map[string]traffic.Report `json:"traffic"`
	Revisions         map[string][]wire.Node    `json:"revisions"`
	AgentSHA256       string                    `json:"agent_sha256"`
	FailedCanaryGated bool                      `json:"failed_canary_gated"`
	RestartRecovered  bool                      `json:"restart_recovered"`
	CancelGated       bool                      `json:"cancel_gated"`
	CleanupSucceeded  bool                      `json:"cleanup_succeeded"`
	Captures          map[string]captureReceipt `json:"captures,omitempty"`
	ExitUpdates       []exitUpdate              `json:"exit_updates,omitempty"`
}

type captureReceipt struct {
	SHA256  string `json:"sha256"`
	Packets int    `json:"packets"`
	Bytes   int    `json:"bytes"`
}

type exitUpdate struct {
	Phase        string `json:"phase"`
	Revision     uint64 `json:"revision,string"`
	StartedAtNS  int64  `json:"started_at_ns"`
	ReadyAtNS    int64  `json:"ready_at_ns"`
	BeforePID    string `json:"before_pid"`
	AfterPID     string `json:"after_pid"`
	TrafficEndNS int64  `json:"traffic_end_ns"`
}

type probeInput struct {
	Clients []probeClient `json:"clients"`
	Phase   string        `json:"phase"`
}

type probeClient struct {
	ID, Engine string
	Port       int
	Config     model.Object
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func eventually(t *testing.T, description string, limit time.Duration, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timed out waiting for " + description)
}
