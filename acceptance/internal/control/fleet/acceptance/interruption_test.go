package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/control/integration/evidence/installation"
	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/fleet/service"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/journal"
	"vpn-script/internal/platform/safeio"
)

type fleetIntent struct {
	Accepted contract.Receipt `json:"accepted"`
	Applied  contract.Receipt `json:"applied"`
	Phase    string           `json:"phase"`
	Attempt  uint64           `json:"attempt,string"`
}

func (f *fixture) interrupt(t *testing.T) {
	previous, err := filepath.EvalSymlinks(filepath.Join(nodeBase, "current"))
	require(t, err)
	proof := &interruptionReport{AgentBefore: property(t, service.Name, "MainPID")}
	proof.CredentialsBefore, err = safeio.FileHash(filepath.Join(agentState, "connection.json"))
	require(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	require(t, installation.Gate(ctx))
	defer func() { require(t, installation.Ungate(context.Background())) }()
	next := f.configuration(t, true)
	proof.Revision = f.publish(t, next)
	pending, err := installation.ObservePending(ctx, previous, filepath.Join(f.root, "reports"))
	require(t, err)
	proof.ControlPID, proof.JournalSHA256 = pending.ControlPID, pending.JournalSHA256
	proof.PreviousRelease, proof.NextRelease = pending.PreviousRelease, pending.NextRelease
	intent, digest := f.intent(t, "interrupted-agent-before.json")
	f.pendingIntent(t, intent, proof)
	proof.IntentBeforeSHA, proof.AttemptBefore = digest, intent.Attempt
	f.killApplyingAgent(t, proof)
	require(t, installation.Ungate(ctx))
	f.resumeInterrupted(t, next, intent, proof)
	f.updated = next
	f.report.Generations = append(f.report.Generations, next.Generation)
	f.report.Interruption = proof
	f.traffic(t, "interrupted-install-recovered")
}

func (f *fixture) intent(t *testing.T, name string) (fleetIntent, string) {
	t.Helper()
	raw, err := safeio.Read(filepath.Join(agentState, "agent/state.json"), 1<<20)
	require(t, err)
	var value fleetIntent
	require(t, json.Unmarshal(raw, &value))
	if name != "" {
		require(t, safeio.Atomic(filepath.Join(f.root, "reports", name), raw, 0600))
	}
	return value, safeio.Hash(raw)
}

func (f *fixture) pendingIntent(t *testing.T, value fleetIntent, proof *interruptionReport) {
	t.Helper()
	node, err := f.controller.client.Node(t.Context(), nodeID, f.controller.admin)
	require(t, err)
	require(t, safeio.NewJSON(filepath.Join(f.root, "reports", "interrupted-desired.json"), node.Desired))
	if value.Phase != "applying" || value.Accepted.Revision != proof.Revision ||
		value.Accepted.PayloadSHA256 != node.Desired.PayloadSHA256 ||
		value.Applied.Revision >= proof.Revision || value.Applied.Revision == 0 || value.Attempt == 0 {
		t.Fatal("Fleet did not durably accept the exact assignment before installation")
	}
	proof.PayloadSHA256 = value.Accepted.PayloadSHA256
}

func (f *fixture) killApplyingAgent(t *testing.T, proof *interruptionReport) {
	t.Helper()
	command(t, "systemctl", "kill", "--kill-whom=main", "--signal=SIGKILL", service.Name)
	eventually(t, "applying agent killed by signal", 2*time.Second, func() bool {
		return property(t, service.Name, "MainPID") == "0" && property(t, service.Name, "ExecMainStatus") == "9" &&
			property(t, service.Name, "Result") == "signal"
	})
	proof.Signal, proof.ExitStatus, proof.UnitResult = "SIGKILL", "9", "signal"
	raw := command(t, "systemctl", "show", "-p", "MainPID", "-p", "ExecMainStatus", "-p", "Result", service.Name)
	for _, field := range []string{"MainPID=0\n", "ExecMainStatus=9\n", "Result=signal\n"} {
		if !bytes.Contains(raw, []byte(field)) {
			t.Fatal("lost the actual killed systemd process state")
		}
	}
	require(t, safeio.Atomic(filepath.Join(f.root, "reports", "interrupted-systemd-killed.txt"), raw, 0600))
	digest, err := safeio.FileHash(filepath.Join(nodeBase, journal.Name))
	require(t, err)
	if digest != proof.JournalSHA256 {
		t.Fatal("agent death changed the pending installer transaction")
	}
	_, after := f.intent(t, "interrupted-agent-killed.json")
	if after != proof.IntentBeforeSHA {
		t.Fatal("agent death changed its durable accepted intent")
	}
}

func (f *fixture) resumeInterrupted(t *testing.T, next model.NodeConfig, before fleetIntent, proof *interruptionReport) {
	t.Helper()
	node := f.waitStatus(t, "ready", proof.Revision)
	if node.Status.AppliedRevision != proof.Revision || node.Status.Generation != next.Generation {
		t.Fatal("restarted agent did not finish the exact interrupted assignment")
	}
	var after fleetIntent
	var digest string
	eventually(t, "ready durable agent receipt", 10*time.Second, func() bool {
		after, digest = f.intent(t, "interrupted-agent-after.json")
		return after.Phase == "ready" && after.Applied == before.Accepted
	})
	proof.IntentAfterSHA, proof.AttemptAfter = digest, after.Attempt
	proof.AgentAfter = property(t, service.Name, "MainPID")
	var err error
	proof.CredentialsAfter, err = safeio.FileHash(filepath.Join(agentState, "connection.json"))
	require(t, err)
	if proof.AgentBefore == "0" || proof.AgentAfter == "0" || proof.AgentBefore == proof.AgentAfter ||
		after.Accepted != before.Accepted || after.Applied != before.Accepted || after.Attempt <= before.Attempt ||
		proof.CredentialsAfter != proof.CredentialsBefore {
		t.Fatal("automatic recovery changed the payload or credentials, or did not restart the agent")
	}
	f.committedInterruption(t, proof)
	f.verifyInstallation(t, next)
	proof.Recovered = true
}

func (f *fixture) committedInterruption(t *testing.T, proof *interruptionReport) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(nodeBase, journal.Name)); !os.IsNotExist(err) {
		t.Fatal("successful Fleet recovery retained the installer transaction")
	}
	var state installer.State
	raw, err := safeio.Read(filepath.Join(nodeBase, "state.json"), 1<<20)
	require(t, err)
	require(t, json.Unmarshal(raw, &state))
	if state.ReleaseID != proof.NextRelease {
		t.Fatal("automatic recovery did not commit the originally staged release")
	}
	require(t, safeio.Atomic(filepath.Join(f.root, "reports", "interrupted-final-state.json"), raw, 0600))
}
