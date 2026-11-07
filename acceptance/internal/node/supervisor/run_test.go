package supervisor_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
	"vpn-script/internal/node/supervisor"
	"vpn-script/internal/platform/safeio"
)

func TestMain(m *testing.M) {
	if os.Getenv("NHOP_SUPERVISOR_HELPER") == "1" {
		helper()
		return
	}
	os.Exit(m.Run())
}

func helper() {
	name := filepath.Base(os.Args[0])
	_ = os.WriteFile(name+".pid", []byte(strconv.Itoa(os.Getpid())), 0600)
	for {
		if _, err := os.Stat("fail-" + name); err == nil {
			os.Exit(7)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSupervisorStopsAllChildren(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) { lifecycle(t, failure) })
	}
}

func lifecycle(t *testing.T, failure bool) {
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
	t.Setenv("NHOP_SUPERVISOR_HELPER", "1")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- supervisor.Run(ctx, directory) }()
	pids := []int{waitPID(t, directory, "xray"), waitPID(t, directory, "sing-box")}
	if failure {
		_ = os.WriteFile(filepath.Join(directory, "fail-xray"), nil, 0600)
	} else {
		cancel()
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("failure/cancellation not reported")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("supervisor did not stop")
	}
	for _, pid := range pids {
		if syscall.Kill(pid, 0) == nil {
			t.Fatalf("orphan engine PID %d", pid)
		}
	}
}

func waitPID(t *testing.T, directory, name string) int {
	t.Helper()
	for range 200 {
		raw, err := os.ReadFile(filepath.Join(directory, name+".pid"))
		if err == nil {
			pid, err := strconv.Atoi(string(raw))
			if err == nil {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("engine did not start")
	return 0
}
