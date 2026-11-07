package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

type supervised struct {
	command *exec.Cmd
	done    chan struct{}
	err     error
	output  bytes.Buffer
	path    string
	pids    []int
}

func startSupervised(t *testing.T, stubborn bool) *supervised {
	t.Helper()
	directory, executable := signalFixture(t)
	s := &supervised{path: directory, done: make(chan struct{})}
	s.command = process.Command(context.Background(), executable, "--supervise", directory)
	s.command.Env = append(os.Environ(), "NHOP_NODE_SIGNAL_HELPER=1")
	if stubborn {
		s.command.Env = append(s.command.Env, "NHOP_NODE_STUBBORN=1")
	}
	s.command.Stdout, s.command.Stderr = &s.output, &s.output
	if err := s.command.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { s.err = s.command.Wait(); close(s.done) }()
	t.Cleanup(func() { _ = process.Stop(s.command); <-s.done })
	s.pids = []int{enginePID(t, directory, "xray"), enginePID(t, directory, "sing-box")}
	return s
}

func (s *supervised) finished(t *testing.T, success bool) {
	t.Helper()
	select {
	case <-s.done:
		if (s.err == nil) != success {
			t.Errorf("unexpected supervisor exit: %v: %s", s.err, s.output.String())
		}
	case <-time.After(8 * time.Second):
		t.Fatal("supervisor exceeded shutdown deadline")
	}
	for _, pid := range s.pids {
		if syscall.Kill(pid, 0) == nil {
			t.Errorf("supervisor left engine PID %d running", pid)
		}
	}
}

func TestSignalsStopSupervisorSuccessfullyAndLetEnginesExit(t *testing.T) {
	for _, signal := range []syscall.Signal{syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP} {
		t.Run(signal.String(), func(t *testing.T) {
			s := startSupervised(t, false)
			if err := s.command.Process.Signal(signal); err != nil {
				t.Fatal(err)
			}
			s.finished(t, true)
			for _, name := range []string{"xray", "sing-box"} {
				if _, err := os.Stat(filepath.Join(s.path, name+".stopped")); err != nil {
					t.Errorf("engine %s was killed without its termination handler", name)
				}
			}
		})
	}
}

func TestShutdownForcesEnginesThatIgnoreTermination(t *testing.T) {
	s := startSupervised(t, true)
	if err := s.command.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	s.finished(t, true)
}

func TestEngineFailureRemainsAFailedSupervisorExit(t *testing.T) {
	s := startSupervised(t, false)
	if err := os.WriteFile(filepath.Join(s.path, "fail-xray"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	s.finished(t, false)
	if !strings.Contains(s.output.String(), "engine xray") {
		t.Fatal("failed engine identity was lost", s.output.String())
	}
	if _, err := os.Stat(filepath.Join(s.path, "sing-box.stopped")); err != nil {
		t.Error("engine failure did not let its peer shut down", err)
	}
}
