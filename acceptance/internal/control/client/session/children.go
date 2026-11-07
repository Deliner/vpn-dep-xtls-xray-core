package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"vpn-script/internal/platform/process"
	"vpn-script/internal/runtime/engines"
)

func (g *children) start(request Request, name, engine string, unprivileged bool) (int, error) {
	log, err := os.OpenFile(filepath.Join(request.Directory, name+".log"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return 0, err
	}
	args := engines.Arguments(engine, filepath.Join(request.Directory, name+".json"), false)
	command := process.Command(context.Background(), filepath.Join(request.Directory, name), args...)
	command.Stdout, command.Stderr = log, log
	command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "HOME=/nonexistent", "LD_LIBRARY_PATH=" + request.Directory}
	credential(command, request.Network.EngineUID, request.EngineGID, unprivileged)
	if err := command.Start(); err != nil {
		log.Close()
		return 0, fmt.Errorf("start %s: %w", name, err)
	}
	g.commands = append(g.commands, command)
	g.waiters.Add(1)
	go g.reap(command, name, log)
	return command.Process.Pid, nil
}

func (g *children) reap(command *exec.Cmd, name string, log *os.File) {
	defer g.waiters.Done()
	err := command.Wait()
	log.Close()
	g.exits <- childExit{name: name, err: err}
	g.notice.Do(func() { close(g.done) })
}

func (g *children) stop() {
	g.once.Do(g.shutdown)
}

func (g *children) shutdown() {
	for _, command := range g.commands {
		_ = process.Terminate(command)
	}
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
