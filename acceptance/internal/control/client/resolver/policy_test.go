package resolver

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func fixture(t *testing.T, run process.Runner) *Manager {
	t.Helper()
	return &Manager{run: run, lookup: func(name string) (*net.Interface, error) {
		return &net.Interface{Name: name, Index: 123}, nil
	}}
}

func TestPartialFailureRevertsWithIndependentContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var rollback bool
	m := fixture(t, func(ctx context.Context, _ time.Duration, args ...string) (process.Result, error) {
		if args[1] == "domain" {
			cancel()
			return process.Result{}, errors.New("synthetic D-Bus refusal")
		}
		if args[1] == "revert" {
			rollback = ctx.Err() == nil
			return process.Result{}, errors.New("synthetic rollback refusal")
		}
		return process.Result{Success: true}, nil
	})
	err := m.ConfigureNewLink(ctx, Link{Name: "nhop0", Index: 123})
	if err == nil || !rollback || !strings.Contains(err.Error(), "D-Bus refusal") || !strings.Contains(err.Error(), "rollback refusal") {
		t.Fatalf("failure or rollback diagnostics lost: %v, independent rollback=%t", err, rollback)
	}
}

func TestReplacedOrForeignLinkIsNeverModified(t *testing.T) {
	m := fixture(t, func(_ context.Context, _ time.Duration, _ ...string) (process.Result, error) {
		t.Fatal("DNS command reached a foreign interface")
		return process.Result{}, nil
	})
	for _, link := range []Link{{"eth0", 123}, {"nhop0", 456}, {"nhop0", 0}} {
		if m.ConfigureNewLink(t.Context(), link) == nil || m.Revert(t.Context(), link) == nil {
			t.Fatal("foreign or replaced interface accepted")
		}
	}
}
