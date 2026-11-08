package fleet

import (
	"context"
	"errors"
	"strings"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/platform/safeio"
)

func TestAgentPersistsReceiptBeforeCallingInstaller(t *testing.T) {
	f := newAgentFixture(t)
	f.agent.install = func(_ context.Context, data model.NodeConfig) (installer.Result, error) {
		journal, err := f.agent.loadState()
		require(t, err)
		if journal.Accepted.Revision != 4 || journal.Accepted.PayloadSHA256 == "" || journal.Phase != "applying" || journal.Applied.Revision != 0 {
			t.Fatalf("installation began before durable intent: %+v", journal)
		}
		return installed(data), nil
	}
	status, err := f.agent.Apply(context.Background(), f.signed(t, 4))
	require(t, err)
	if status.Phase != "ready" || status.AppliedRevision != 4 || status.Generation != f.data.Generation || status.Attempt != 1 {
		t.Fatalf("incorrect successful application status: %+v", status)
	}
	journal, err := f.agent.loadState()
	require(t, err)
	if journal.Applied != journal.Accepted {
		t.Fatal("successful installer result not committed")
	}
}

func TestFailedInstallRetainsLastAppliedRevisionAndPreventsRollback(t *testing.T) {
	f := newAgentFixture(t)
	_, err := f.agent.Apply(context.Background(), f.signed(t, 1))
	require(t, err)
	f.agent.install = func(context.Context, model.NodeConfig) (installer.Result, error) {
		return installer.Result{}, errors.New("private-token-fixture")
	}
	status, err := f.agent.Apply(context.Background(), f.signed(t, 2))
	if err == nil || status.Phase != "failed" || status.AcceptedRevision != 2 || status.AppliedRevision != 1 || status.Attempt != 2 {
		t.Fatalf("failed installation lost history: %+v, %v", status, err)
	}
	raw, err := safeio.Encode(status)
	require(t, err)
	if strings.Contains(string(raw), "private-token-fixture") {
		t.Fatal("status leaked raw installer error")
	}
	agent := f.restarted(t)
	agent.install = func(_ context.Context, data model.NodeConfig) (installer.Result, error) { return installed(data), nil }
	if _, err := agent.Apply(context.Background(), f.signed(t, 1)); err == nil {
		t.Fatal("old revision accepted after failed install and restart")
	}
	status, err = agent.Apply(context.Background(), f.signed(t, 2))
	require(t, err)
	if status.Phase != "ready" || status.AppliedRevision != 2 || status.Attempt != 3 {
		t.Fatalf("exact retry did not recover: %+v", status)
	}
}

func TestInterruptedInstallRetriesExactPayloadAfterAgentRestart(t *testing.T) {
	f := newAgentFixture(t)
	f.agent.install = func(context.Context, model.NodeConfig) (installer.Result, error) { panic("installer interrupted") }
	simulateInterruptedInstall(t, f.agent, f.signed(t, 2))
	agent := f.restarted(t)
	calls := 0
	agent.install = func(_ context.Context, data model.NodeConfig) (installer.Result, error) {
		calls++
		return installed(data), nil
	}
	if _, err := agent.Apply(context.Background(), f.signed(t, 1)); err == nil {
		t.Fatal("restart accepted an older revision")
	}
	status, err := agent.Apply(context.Background(), f.signed(t, 2))
	require(t, err)
	if calls != 1 || status.AppliedRevision != 2 || status.Phase != "ready" || status.Attempt != 2 {
		t.Fatalf("interrupted intent not recovered exactly once: calls=%d, status=%+v", calls, status)
	}
}

func simulateInterruptedInstall(t *testing.T, agent *Agent, raw []byte) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != "installer interrupted" {
			t.Fatalf("unexpected interruption: %v", recovered)
		}
	}()
	_, err := agent.Apply(context.Background(), raw)
	t.Fatalf("installer was not reached: %v", err)
}
