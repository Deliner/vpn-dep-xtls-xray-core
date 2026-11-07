package diagnostics

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"time"

	"vpn-script/internal/platform/process"
)

// CaptureLoopback records test UDP packets in the explicitly isolated Linux VM.
// The caller owns stop and must call it before copying the report directory.
func CaptureLoopback(path string) (func() error, error) {
	output, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	var diagnostic bytes.Buffer
	command := process.Command(context.Background(), "tcpdump", "-nn", "-U", "-i", "lo", "-s", "256", "-w", "-", "udp")
	command.Stdout, command.Stderr = output, &diagnostic
	if err := command.Start(); err != nil {
		_ = output.Close()
		return nil, err
	}
	var once sync.Once
	var result error
	return func() error {
		once.Do(func() {
			result = errors.Join(stopCapture(command), output.Close(), os.WriteFile(path+".log", diagnostic.Bytes(), 0600))
		})
		return result
	}, nil
}

func stopCapture(command *exec.Cmd) error {
	_ = process.Terminate(command)
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer process.Stop(command)
	select {
	case err := <-done:
		return err
	case <-time.After(2 * time.Second):
		_ = process.Stop(command)
		return <-done
	}
}
