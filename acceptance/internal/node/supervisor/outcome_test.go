package supervisor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCancellationWinsWhenExitNotificationIsAlsoReady(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for range 100 {
		exits := make(chan exitResult, 1)
		exits <- exitResult{name: "xray", err: errors.New("stopped during shutdown")}
		if err := waitForExit(ctx, exits); !errors.Is(err, context.Canceled) {
			t.Fatalf("shutdown became an engine failure: %v", err)
		}
	}
}

func TestDeadlineAndUnexpectedEngineExitRemainErrors(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	if err := waitForExit(ctx, make(chan exitResult)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("deadline was lost", err)
	}
	for _, cause := range []error{nil, errors.New("exit status 7")} {
		exits := make(chan exitResult, 1)
		exits <- exitResult{name: "sing-box", err: cause}
		err := waitForExit(t.Context(), exits)
		if err == nil || !strings.Contains(err.Error(), "engine sing-box") || cause != nil && !errors.Is(err, cause) {
			t.Fatal("unexpected engine exit was hidden", err)
		}
	}
}
