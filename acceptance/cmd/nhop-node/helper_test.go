package main

import (
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/platform/safeio"
)

func TestMain(m *testing.M) {
	if os.Getenv("NHOP_NODE_SIGNAL_HELPER") == "1" {
		name := filepath.Base(os.Args[0])
		if name == "xray" || name == "sing-box" {
			engineHelper(name)
		} else {
			main()
		}
		return
	}
	os.Exit(m.Run())
}

func engineHelper(name string) {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)
	if os.Getenv("NHOP_NODE_STUBBORN") == "1" {
		signal.Ignore(syscall.SIGTERM)
	}
	if os.WriteFile(name+".pid", []byte(strconv.Itoa(os.Getpid())), 0600) != nil {
		os.Exit(9)
	}
	for {
		select {
		case <-stop:
			if os.WriteFile(name+".stopped", []byte("terminated\n"), 0600) != nil {
				os.Exit(8)
			}
			return
		case <-time.After(10 * time.Millisecond):
			if _, err := os.Stat("fail-" + name); err == nil {
				os.Exit(7)
			}
		}
	}
}

func signalFixture(t *testing.T) (string, string) {
	t.Helper()
	directory := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"xray", "sing-box"} {
		if err := os.Symlink(executable, filepath.Join(directory, name)); err != nil {
			t.Fatal(err)
		}
	}
	if err := safeio.JSON(filepath.Join(directory, "processes.json"), []string{"xray", "sing-box"}); err != nil {
		t.Fatal(err)
	}
	return directory, executable
}

func enginePID(t *testing.T, directory, name string) int {
	t.Helper()
	for range 300 {
		raw, err := os.ReadFile(filepath.Join(directory, name+".pid"))
		if err == nil {
			if pid, err := strconv.Atoi(string(raw)); err == nil && pid > 0 {
				t.Cleanup(func() { _ = syscall.Kill(-pid, syscall.SIGKILL) })
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("engine did not become ready")
	return 0
}
