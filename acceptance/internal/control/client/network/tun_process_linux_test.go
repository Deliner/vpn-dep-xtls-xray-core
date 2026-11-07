package network

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func liveProcess(t *testing.T, directory, label string, arguments ...string) func() {
	t.Helper()
	log, err := os.Create(filepath.Join(directory, label+".log"))
	if err != nil {
		t.Fatal(err)
	}
	command := process.Command(context.Background(), arguments[0], arguments[1:]...)
	command.Env = append(os.Environ(), "LD_LIBRARY_PATH="+directory)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		log.Close()
		t.Fatal(err)
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			process.Stop(command)
			command.Wait()
			log.Close()
		})
	}
	t.Cleanup(func() {
		stop()
		raw, err := os.ReadFile(filepath.Join(directory, label+".log"))
		if err != nil {
			t.Error(err)
		}
		t.Logf("%s: %s", label, raw)
	})
	return stop
}

func waitLiveCommand(t *testing.T, expected string, args ...string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		command := exec.Command("ip", append([]string{"netns", "exec", "nhop-client-inner"}, args...)...)
		output, err := command.CombinedOutput()
		if err == nil && strings.Contains(string(output), expected) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("client did not become ready: %v", args)
}
