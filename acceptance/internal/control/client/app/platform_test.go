package app_test

import (
	"bytes"
	"runtime"
	"strings"
	"testing"

	"vpn-script/internal/control/client/app"
)

func TestActivationCommandsRefuseMacBeforeReadingInputs(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS host safety contract")
	}
	for _, args := range [][]string{{"start", "--profile", "/does/not/exist", "--runtimes", "/does/not/exist"}, {"stop"}, {"diagnostics"}, {"reconnect"}, {"resume"}} {
		var output bytes.Buffer
		err := app.Run(args, &output)
		if err == nil || !strings.Contains(err.Error(), "Linux root") || output.Len() != 0 {
			t.Fatalf("activation was not rejected before host access: %v %q", err, output.String())
		}
	}
}
