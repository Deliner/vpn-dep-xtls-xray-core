package masque

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func startManaged(t *testing.T, directory, label, namespace string, arguments ...string) *managed {
	t.Helper()
	if namespace != "" {
		arguments = append([]string{"ip", "netns", "exec", namespace}, arguments...)
	}
	log, err := os.OpenFile(filepath.Join(directory, label+".log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	command := process.Command(context.Background(), arguments[0], arguments[1:]...)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		log.Close()
		t.Fatal(err)
	}
	value := &managed{command: command, done: make(chan error, 1), log: log}
	go func() {
		value.done <- command.Wait()
		close(value.done)
	}()
	t.Cleanup(func() { value.stop(t) })
	return value
}

func (m *managed) stop(t *testing.T) {
	t.Helper()
	if m.stopped {
		return
	}
	_ = process.Terminate(m.command)
	var err error
	select {
	case err = <-m.done:
	case <-time.After(3 * time.Second):
		_ = process.Stop(m.command)
		err = <-m.done
	}
	m.stopped = true
	if m.cleanExit && err != nil {
		t.Errorf("origin helper did not close cleanly: %v", err)
	}
	if err := m.log.Close(); err != nil {
		t.Fatal(err)
	}
}

func awaitFile(t *testing.T, child *managed, path string) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if data, err := os.ReadFile(path); err == nil && string(data) == "ready\n" {
			return
		}
		select {
		case err := <-child.done:
			t.Fatalf("origin exited before readiness: %v", err)
		case <-deadline.C:
			t.Fatal("origin readiness timeout")
		case <-ticker.C:
		}
	}
}
