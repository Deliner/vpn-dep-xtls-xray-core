package acceptance

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/fleet/service"
)

func (f *fixture) rollback(t *testing.T) {
	before, err := os.ReadFile(filepath.Join(nodeBase, "state.json"))
	require(t, err)
	pid := property(t, "vpn-nhop.service", "MainPID")
	bad := f.configuration(t, false)
	for _, program := range bad.Programs {
		if program.Name == "sing-box" {
			program.Config["log"] = model.Object{"level": "info", "output": filepath.Join(f.root, "denied-vpn.log")}
		}
	}
	revision := f.publish(t, digestConfiguration(t, bad))
	node := f.waitStatus(t, "failed", revision)
	f.cli(t, "agent-service-stop")
	after, err := os.ReadFile(filepath.Join(nodeBase, "state.json"))
	require(t, err)
	if !bytes.Equal(before, after) || node.Status.AppliedRevision != revision-1 || node.Status.Generation != f.updated.Generation || property(t, "vpn-nhop.service", "MainPID") == pid {
		t.Fatal("failed activation did not exercise and restore the previous VPN service")
	}
	f.traffic(t, "rolled-back-agent-stopped")
	f.report.Rollback, f.report.AgentStopRetainsVPN = true, true
	revision = f.publish(t, f.updated)
	f.cli(t, "agent-service-start")
	f.waitStatus(t, "ready", revision)
	f.verifyInstallation(t, f.updated)
}

func (f *fixture) outage(t *testing.T) {
	before, err := f.controller.client.Node(t.Context(), nodeID, f.controller.admin)
	require(t, err)
	pid := property(t, "vpn-nhop.service", "MainPID")
	f.controller.close(t)
	time.Sleep(3 * time.Second)
	f.traffic(t, "controller-offline")
	if property(t, "vpn-nhop.service", "MainPID") != pid {
		t.Fatal("controller outage restarted the VPN")
	}
	f.controller.reopen(t)
	eventually(t, "controller recovery heartbeat", time.Minute, func() bool {
		node, err := f.controller.client.Node(t.Context(), nodeID, f.controller.admin)
		return err == nil && node.Status != nil && node.Status.Phase == "ready" && node.Status.Attempt > before.Status.Attempt
	})
	f.verifyInstallation(t, f.updated)
	f.report.ControllerRecovery = true
}

func (f *fixture) crash(t *testing.T) {
	path := filepath.Join(agentState, "connection.json")
	before, err := os.ReadFile(path)
	require(t, err)
	node, err := f.controller.client.Node(t.Context(), nodeID, f.controller.admin)
	require(t, err)
	agentPID, vpnPID := property(t, service.Name, "MainPID"), property(t, "vpn-nhop.service", "MainPID")
	command(t, "systemctl", "kill", "--kill-whom=main", "--signal=SIGKILL", service.Name)
	eventually(t, "agent restart after SIGKILL", 20*time.Second, func() bool {
		pid := property(t, service.Name, "MainPID")
		return pid != "0" && pid != agentPID && property(t, service.Name, "SubState") == "running"
	})
	eventually(t, "fresh heartbeat from restarted agent", time.Minute, func() bool {
		current, err := f.controller.client.Node(t.Context(), nodeID, f.controller.admin)
		return err == nil && current.Status != nil && current.Status.Phase == "ready" && current.Status.Attempt > node.Status.Attempt
	})
	after, err := os.ReadFile(path)
	require(t, err)
	if !bytes.Equal(before, after) || property(t, "vpn-nhop.service", "MainPID") != vpnPID {
		t.Fatal("agent crash changed enrollment credentials or restarted the VPN")
	}
	f.traffic(t, "agent-restarted")
	f.report.AgentCrashRecovery, f.report.CredentialsUnchanged = true, true
}

func (f *fixture) revoke(t *testing.T) {
	require(t, f.controller.client.Revoke(t.Context(), nodeID, f.controller.admin))
	eventually(t, "revoked agent exit", 20*time.Second, func() bool {
		return property(t, service.Name, "ExecMainStatus") == "78" && property(t, service.Name, "MainPID") == "0"
	})
	restarts := property(t, service.Name, "NRestarts")
	time.Sleep(6 * time.Second) // Longer than the unit's RestartSec.
	if property(t, service.Name, "NRestarts") != restarts || property(t, service.Name, "MainPID") != "0" {
		t.Fatal("revoked agent was restarted by systemd")
	}
	f.traffic(t, "revoked-agent-vpn-retained")
	f.report.RevokeExitStatus, f.report.RevokeNoRestart = "78", true
}
