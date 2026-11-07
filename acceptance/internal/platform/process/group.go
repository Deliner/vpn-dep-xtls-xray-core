//go:build darwin || linux

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Command gives an owned subprocess its own process group. Cancellation kills
// the group, including ordinary descendants that inherit its sockets and pipes.
// Call Stop after Wait/Run as well, to collect background work left on normal exit.
func Command(ctx context.Context, binary string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, binary, args...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	var once sync.Once
	var stopError error
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		once.Do(func() { stopError = killGroup(command.Process.Pid) })
		return stopError
	}
	command.WaitDelay = time.Second
	return command
}

// Stop terminates the process group created by Command. It does not replace Wait.
// Deliberate setsid/setpgid escapes require an OS service/cgroup boundary.
func Stop(command *exec.Cmd) error {
	if command.Process == nil || command.Cancel == nil {
		return os.ErrProcessDone
	}
	return command.Cancel()
}

// Terminate requests a cooperative shutdown of the group owned by Command.
// The caller must still Wait and eventually Stop to enforce its shutdown bound.
func Terminate(command *exec.Cmd) error {
	if command.Process == nil || command.SysProcAttr == nil || !command.SysProcAttr.Setpgid || command.SysProcAttr.Pgid != 0 {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}

func killGroup(pid int) error {
	err := syscall.Kill(-pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
