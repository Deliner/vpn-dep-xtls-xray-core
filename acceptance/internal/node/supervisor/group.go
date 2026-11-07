package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"vpn-script/internal/platform/process"
	"vpn-script/internal/runtime/engines"
)

type processGroup struct {
	commands []*exec.Cmd
	exits    chan exitResult
	waiters  sync.WaitGroup
}

type exitResult struct {
	name string
	err  error
}

func start(ctx context.Context, directory string, names []string) (*processGroup, error) {
	group := &processGroup{exits: make(chan exitResult, len(names))}
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			group.stop()
			return nil, err
		}
		// The supervisor owns shutdown policy; parent cancellation must not send SIGKILL first.
		command := process.Command(context.WithoutCancel(ctx), filepath.Join(directory, name), engines.Arguments(name, filepath.Join(directory, name+".json"), false)...)
		command.Stdout, command.Stderr, command.Dir = os.Stdout, os.Stderr, directory
		if err := command.Start(); err != nil {
			group.stop()
			return nil, fmt.Errorf("start %s: %w", name, err)
		}
		group.commands = append(group.commands, command)
		group.waiters.Add(1)
		go group.reap(command, name)
	}
	return group, nil
}

func (g *processGroup) reap(command *exec.Cmd, name string) {
	defer g.waiters.Done()
	g.exits <- exitResult{name: name, err: command.Wait()}
}

func (g *processGroup) stop() {
	for _, command := range g.commands {
		_ = process.Terminate(command)
	}
	// Completion notices and waiter ownership are separate: Run may already have
	// consumed the first notice, but every started child must still be reaped.
	done := make(chan struct{})
	go func() { g.waiters.Wait(); close(done) }()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	}
	for _, command := range g.commands {
		_ = process.Stop(command)
	}
	<-done
}
