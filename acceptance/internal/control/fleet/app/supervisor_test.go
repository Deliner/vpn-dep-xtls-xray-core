package app

import (
	"context"
	"io"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func TestFleetBinaryKeepsInstalledSupervisorEntrypoint(t *testing.T) {
	root := privateDirectory(t)
	require(t, safeio.NewJSON(filepath.Join(root, "processes.json"), []string{"unsupported-engine"}))
	err := Run(context.Background(), []string{"--supervise", root}, io.Discard)
	expected := "invalid process name"
	if runtime.GOOS != "linux" {
		expected = "requires Linux"
	}
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("installed supervisor dispatch lost: %v", err)
	}
}
