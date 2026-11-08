package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShutdownReapsRemainingChildAfterExitNoticeWasConsumed(t *testing.T) {
	directory := t.TempDir()
	for name, script := range map[string]string{
		"native":   "#!/bin/sh\nexit 0\n",
		"frontend": "#!/bin/sh\ntrap '' TERM\nwhile :; do sleep 1; done\n",
	} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	group := newChildren()
	t.Cleanup(group.stop)
	request := Request{Directory: directory}
	for _, name := range []string{"native", "frontend"} {
		if _, err := group.start(request, name, "xray", false); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case exit := <-group.exits:
		if exit.name != "native" || exit.err != nil {
			t.Fatalf("unexpected first child: %+v", exit)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("native test process was not reaped")
	}
	group.stop()
	for _, command := range group.commands {
		if command.ProcessState == nil {
			t.Fatal("shutdown returned before reaping a child")
		}
	}
	group.stop() // Repeated cleanup must not signal potentially reused PIDs.
}

func TestReconnectLogsAppendWithoutAdoptingSymlinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native.log")
	for _, line := range []string{"first\n", "second\n"} {
		log, err := childLog(path)
		if err != nil {
			t.Fatal(err)
		}
		_, err = log.WriteString(line)
		closeErr := log.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "first\nsecond\n" {
		t.Fatal("reconnect lost prior process diagnostics", err)
	}
	alias := filepath.Join(filepath.Dir(path), "alias.log")
	if err := os.Symlink(path, alias); err != nil {
		t.Fatal(err)
	}
	if log, err := childLog(alias); err == nil {
		log.Close()
		t.Fatal("log reopen followed a symlink")
	}
}
