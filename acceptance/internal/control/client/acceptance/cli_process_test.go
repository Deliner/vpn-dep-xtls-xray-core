package acceptance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/control/client/app"
	clientcontrol "vpn-script/internal/control/client/controller"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func TestPublicClientHelper(t *testing.T) {
	if os.Getenv("NHOP_PUBLIC_CLIENT_HELPER") != "1" {
		t.Skip("isolated public CLI subprocess")
	}
	for index, argument := range os.Args {
		if argument == "--" {
			if err := app.Run(os.Args[index+1:], os.Stdout); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	t.Fatal("missing CLI arguments")
}

func crashGuard(t *testing.T, profile, runtimes, stage string) clientcontrol.Diagnostic {
	t.Helper()
	directory, real := faultTool(t)
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	command := cliContext(t, ctx, "start", "--profile", profile, "--runtimes", runtimes, "--ipv6", os.Getenv("NHOP_CLIENT_IPV6"), "--dns-ip", "198.18.0.2", "--dns-name", "dns.example.invalid")
	command.Env = append(command.Env, "PATH="+directory+":"+os.Getenv("PATH"), "NHOP_NFT_REAL="+real, "NHOP_NFT_FAULT_STAGE="+stage, "NHOP_NFT_FAULT_MARKER="+filepath.Join(directory, "marker"))
	_, err := command.Output()
	process.Stop(command)
	marker, readErr := os.ReadFile(filepath.Join(directory, "marker"))
	if err == nil || ctx.Err() != nil || readErr != nil || string(marker) != stage+"\n" {
		t.Fatal("controller did not crash at the requested nft boundary", err, ctx.Err(), readErr)
	}
	t.Cleanup(func() {
		if _, err := cliCommand(t, "stop").Output(); err != nil {
			t.Error("could not release owned planned guard", err)
		}
	})
	return cliCall(t, "diagnostics")
}

func faultTool(t *testing.T) (string, string) {
	t.Helper()
	checkIsolation(t)
	real, err := exec.LookPath("nft")
	if err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp("/run/nhop-session-test", "nft-fault-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(directory) })
	if err := safeio.Create(filepath.Join(directory, "nft"), []byte(nftFaultWrapper), 0755); err != nil {
		t.Fatal(err)
	}
	return directory, real
}

const nftFaultWrapper = `#!/bin/bash
set -euo pipefail
if [[ "$1" == -f ]]; then
  [[ "$(readlink /proc/self/ns/net)" != "$(readlink /proc/1/ns/net)" ]]
  case "$NHOP_NFT_FAULT_STAGE" in
    before) ;;
    after) "$NHOP_NFT_REAL" "$@" ;;
    *) exit 2 ;;
  esac
  printf '%s\n' "$NHOP_NFT_FAULT_STAGE" > "$NHOP_NFT_FAULT_MARKER"
  kill -KILL "$PPID"
  exit 89
fi
exec "$NHOP_NFT_REAL" "$@"
`

func cliCommand(t *testing.T, args ...string) *exec.Cmd {
	return cliContext(t, context.Background(), args...)
}

func cliContext(t *testing.T, ctx context.Context, args ...string) *exec.Cmd {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	arguments := []string{"ip", "netns", "exec", "nhop-client-inner", executable, "-test.run=^TestPublicClientHelper$", "--"}
	if binary := os.Getenv("NHOP_CLIENT_BINARY"); binary != "" {
		arguments = []string{"ip", "netns", "exec", "nhop-client-inner", binary}
	}
	command := process.Command(ctx, arguments[0], append(arguments[1:], args...)...)
	command.Env = append(os.Environ(), "NHOP_PUBLIC_CLIENT_HELPER=1")
	command.Stderr = os.Stderr
	return command
}

func cliCall(t *testing.T, operation string) clientcontrol.Diagnostic {
	t.Helper()
	command := cliCommand(t, operation)
	data, err := command.Output()
	if err != nil {
		t.Fatalf("client %s: %v %s", operation, err, data)
	}
	var value clientcontrol.Diagnostic
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("invalid CLI diagnostic: %v %s", err, data)
	}
	return value
}

func cliStart(t *testing.T, profile, runtimes string) (*exec.Cmd, <-chan error, clientcontrol.Diagnostic) {
	t.Helper()
	return cliForeground(t, "start", "--profile", profile, "--runtimes", runtimes, "--ipv6", os.Getenv("NHOP_CLIENT_IPV6"),
		"--dns-ip", "198.18.0.2", "--dns-name", "dns.example.invalid")
}

func cliForeground(t *testing.T, args ...string) (*exec.Cmd, <-chan error, clientcontrol.Diagnostic) {
	t.Helper()
	command := cliCommand(t, args...)
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	var value clientcontrol.Diagnostic
	if err := json.NewDecoder(output).Decode(&value); err != nil {
		process.Stop(command)
		command.Wait()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		defer close(done)
		done <- command.Wait()
	}()
	t.Cleanup(func() {
		cliCall(t, "stop")
		cliWait(t, done, false)
		sessionLogs(t, value.Directory)
	})
	return command, done, value
}

func cliRejects(t *testing.T, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := cliContext(t, ctx, args...).Output()
	if err == nil || ctx.Err() != nil {
		t.Fatalf("command did not refuse promptly: %v, %v, %s", args, err, data)
	}
}

func cliWait(t *testing.T, done <-chan error, success bool) {
	t.Helper()
	select {
	case err := <-done:
		if success && err != nil {
			t.Fatal("controller exited unsuccessfully", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("controller did not exit")
	}
}
