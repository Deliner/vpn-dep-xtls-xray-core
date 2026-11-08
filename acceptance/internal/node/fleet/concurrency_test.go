package fleet

import (
	"context"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
)

func TestAgentSerializesInstallationAcrossInstances(t *testing.T) {
	f := newAgentFixture(t)
	entered, release, result := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	defer finishInstallation(t, release, result)
	f.agent.install = func(_ context.Context, data model.NodeConfig) (installer.Result, error) {
		close(entered)
		<-release
		return installed(data), nil
	}
	raw := f.signed(t, 1)
	go func() {
		defer close(result)
		_, err := f.agent.Apply(context.Background(), raw)
		result <- err
	}()
	select {
	case <-entered:
	case err := <-result:
		t.Fatalf("first install failed before acquiring ownership: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("first install did not start")
	}
	other := f.restarted(t)
	calls := 0
	other.install = func(_ context.Context, data model.NodeConfig) (installer.Result, error) {
		calls++
		return installed(data), nil
	}
	if _, err := other.Apply(context.Background(), raw); err == nil || calls != 0 {
		t.Fatalf("second agent installed concurrently: calls=%d err=%v", calls, err)
	}
}

func finishInstallation(t *testing.T, release chan struct{}, result chan error) {
	t.Helper()
	close(release)
	select {
	case err := <-result:
		require(t, err)
	case <-time.After(5 * time.Second):
		t.Error("installer did not finish before fixture cleanup")
	}
}
