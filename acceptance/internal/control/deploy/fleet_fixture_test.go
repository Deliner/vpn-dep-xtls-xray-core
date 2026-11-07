package deploy

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/control/bundle"
	"vpn-script/internal/control/topology"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
)

func fleetFixture(t testing.TB, branches, hops int) string {
	t.Helper()
	return renderFleet(t, fleetDefinition(t, branches, hops))
}

func fleetDefinition(t testing.TB, branches, hops int) model.Definition {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	ssh := model.SSH{User: "root", Port: 22222, IdentityFile: filepath.Join(home, ".ssh-custom/key"), KnownHostsFile: filepath.Join(home, ".ssh-custom/hosts")}
	if err := os.MkdirAll(filepath.Dir(ssh.IdentityFile), 0700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{ssh.IdentityFile, ssh.KnownHostsFile} {
		if err := os.WriteFile(path, []byte("owned fake SSH fixture\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	definition := model.Definition{Version: 1, Name: "deployment", Nodes: map[string]*model.Node{}, Routes: map[string]*model.Route{}}
	for branch := range branches {
		route := &model.Route{Client: model.Client{SOCKSPort: 10808 + branch}}
		for hop := range hops {
			id := fmt.Sprintf("branch%02d-hop%02d", branch, hop)
			definition.Nodes[id] = &model.Node{Address: fmt.Sprintf("192.0.2.%d", branch*hops+hop+1), SSH: ssh,
				Services: map[string]*model.Service{"vpn": {Protocol: model.Shadowsocks, Port: 24443}}}
			route.Path = append(route.Path, id+".vpn")
		}
		definition.Routes[fmt.Sprintf("route%02d", branch)] = route
	}
	return definition
}

func renderFleet(t testing.TB, definition model.Definition) string {
	t.Helper()
	top, err := topology.Build(definition)
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(t.TempDir(), "bundle")
	if err := bundle.Render(top, directory, workflowInstallers(t)); err != nil {
		t.Fatal(err)
	}
	return directory
}

func fleetNode(args []string) int {
	for _, argument := range args {
		if strings.HasPrefix(argument, "root@192.0.2.") {
			var number int
			fmt.Sscanf(argument, "root@192.0.2.%d", &number)
			return number - 1
		}
	}
	return -1
}

func successfulRemote(ctx context.Context, delay time.Duration, args ...string) (process.Result, error) {
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return process.Result{}, ctx.Err()
		case <-timer.C:
		}
	}
	result := process.Result{Success: true}
	script := args[len(args)-1]
	if script == "uname -m" {
		result.Output = []byte("aarch64\n")
	} else if strings.Contains(script, "mktemp") {
		result.Output = []byte("/var/tmp/vpn-nhop-AbCd1234\n")
	}
	return result, nil
}

type remoteGates struct {
	started  chan int
	cleaning chan int
	apply    []chan struct{}
	cleanup  []chan struct{}
}

func newRemoteGates(t *testing.T, nodes int) *remoteGates {
	t.Helper()
	gates := &remoteGates{started: make(chan int, nodes), cleaning: make(chan int, nodes)}
	for range nodes {
		gates.apply = append(gates.apply, make(chan struct{}, 1))
		gates.cleanup = append(gates.cleanup, make(chan struct{}, 1))
	}
	return gates
}

func (gates *remoteGates) unblock() {
	for index := range gates.apply {
		close(gates.apply[index])
		close(gates.cleanup[index])
	}
}

func (gates *remoteGates) run(ctx context.Context, _ time.Duration, args ...string) (process.Result, error) {
	id, script := fleetNode(args), args[len(args)-1]
	var permit chan struct{}
	if script == "uname -m" {
		gates.started <- id
		permit = gates.apply[id]
	} else if strings.Contains(script, "'rm' '-rf'") {
		gates.cleaning <- id
		permit = gates.cleanup[id]
	}
	if permit != nil {
		select {
		case <-ctx.Done():
			return process.Result{}, ctx.Err()
		case <-permit:
		}
	}
	return successfulRemote(ctx, 0, args...)
}

type runningFleet struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func startFleet(t *testing.T, directory string, jobs int, gates *remoteGates, runner process.Runner, progress io.Writer) *runningFleet {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	running := &runningFleet{cancel: cancel, done: make(chan struct{})}
	go func() {
		running.err = RunConcurrent(ctx, directory, runner, progress, jobs)
		close(running.done)
	}()
	t.Cleanup(func() {
		cancel()
		gates.unblock()
		select {
		case <-running.done:
		case <-time.After(5 * time.Second):
			t.Error("deployment workers did not stop")
		}
	})
	return running
}

func fleetEvent(t *testing.T, events <-chan int) int {
	t.Helper()
	select {
	case id := <-events:
		return id
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for deployment phase")
		return -1
	}
}

func fleetResult(t *testing.T, running *runningFleet) error {
	t.Helper()
	select {
	case <-running.done:
		return running.err
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for deployment result")
		return nil
	}
}

func noFleetEvent(t *testing.T, events <-chan int) {
	t.Helper()
	select {
	case id := <-events:
		t.Fatalf("unexpected node operation: %d", id)
	case <-time.After(25 * time.Millisecond):
	}
}
