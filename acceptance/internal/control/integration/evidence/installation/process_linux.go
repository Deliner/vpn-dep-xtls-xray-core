package installation

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"vpn-script/internal/platform/process"
)

// KillInstaller terminates only the actual installer process, without its error
// handler or rollback. systemd owns the blocked service; recovery must stop it.
func KillInstaller(ctx context.Context, args []string, previous, output string) (Report, error) {
	if len(args) == 0 {
		return Report{}, fmt.Errorf("missing installer command")
	}
	log, err := os.OpenFile(filepath.Join(output, "interrupted-installer.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return Report{}, err
	}
	defer log.Close()
	command := process.Command(ctx, args[0], args[1:]...)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		return Report{}, err
	}
	defer collect(command)
	gateContext, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	report, err := ObservePending(gateContext, previous, output)
	if err != nil {
		return report, err
	}
	report.InstallerPID = command.Process.Pid
	if err := command.Process.Kill(); err != nil {
		return report, err
	}
	if err := killed(command); err != nil {
		return report, err
	}
	report.Signal = "SIGKILL"
	return report, nil
}

func collect(command *exec.Cmd) {
	_ = process.Stop(command)
	if command.ProcessState == nil {
		_ = command.Wait()
	}
}

func killed(command *exec.Cmd) error {
	err := command.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return fmt.Errorf("installer did not exit by signal: %v", err)
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		return fmt.Errorf("installer exit was not SIGKILL: %v", err)
	}
	return nil
}
