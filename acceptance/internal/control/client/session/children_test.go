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
	group := &children{exits: make(chan childExit, 2)}
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
