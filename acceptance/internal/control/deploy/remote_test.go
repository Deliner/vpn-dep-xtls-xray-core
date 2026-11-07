package deploy

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
)

func manifest() model.Deployment {
	return model.Deployment{Nodes: map[string]model.DeployNode{"exit": {Address: "203.0.113.30",
		SSH: model.SSH{User: "admin", Port: 22022, IdentityFile: "/keys/key", KnownHostsFile: "/keys/hosts"}}},
		Installers: map[string]model.Binary{"aarch64": {File: "installers/nhop-node-linux-arm64"}}}
}

func remoteRunner(calls *[]string, failCopy bool) process.Runner {
	return func(_ context.Context, _ time.Duration, args ...string) (process.Result, error) {
		line := strings.Join(args, " ")
		*calls = append(*calls, line)
		if args[0] == "scp" && failCopy {
			return process.Result{}, fmt.Errorf("injected transfer failure")
		}
		result := process.Result{Success: true}
		if args[len(args)-1] == "uname -m" {
			result.Output = []byte("aarch64\n")
		}
		if strings.Contains(line, "mktemp") {
			result.Output = []byte("/var/tmp/vpn-nhop-AbCd1234\n")
		}
		return result, nil
	}
}

func TestRemoteArchitecturePortsAndCleanup(t *testing.T) {
	var calls []string
	err := applyNode(context.Background(), remoteRunner(&calls, false), "/bundle", "exit", manifest())
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(calls, "\n")
	for _, want := range []string{"ssh -p 22022", "scp -P 22022", "nhop-node-linux-arm64", "StrictHostKeyChecking=yes", "'sudo' '-n' '--'", "'--config'", "mktemp -d /var/tmp/"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing remote behavior: %s", want)
		}
	}
	if !strings.Contains(calls[len(calls)-1], "'rm' '-rf' '--' '/var/tmp/vpn-nhop-AbCd1234'") {
		t.Fatal("private staging directory was not cleaned up")
	}
}

func TestTransferFailureStillCleansUp(t *testing.T) {
	var calls []string
	err := applyNode(context.Background(), remoteRunner(&calls, true), "/bundle", "exit", manifest())
	if err == nil || !strings.Contains(calls[len(calls)-1], "'rm' '-rf'") {
		t.Fatal("failed transfer did not return an error and clean up")
	}
	for _, call := range calls {
		if strings.Contains(call, "'sudo'") {
			t.Fatal("installer ran after a failed upload")
		}
	}
}
