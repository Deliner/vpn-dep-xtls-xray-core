package systemd_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/control/client/systemd/manager"
)

type bootFixture struct {
	t                   *testing.T
	root, binary, state string
	guard, consumer     string
	child               *exec.Cmd
	command             manager.Command
	files               map[string][]byte
	mounted             bool
}

func (f *bootFixture) control(success bool, args ...string) []byte {
	f.t.Helper()
	data, err := f.command(f.t.Context(), args...)
	if (err == nil) != success {
		f.t.Fatalf("unexpected systemd result %v: %v %s", args, err, data)
	}
	return data
}

func (f *bootFixture) cli(args ...string) []byte {
	f.t.Helper()
	return f.run(true, "ip", append([]string{"netns", "exec", "nhop-client-inner", f.binary}, args...)...)
}

func (f *bootFixture) run(success bool, name string, args ...string) []byte {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(f.t.Context(), 10*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if (err == nil) != success {
		f.t.Fatalf("unexpected %s result: %v %s", name, err, data)
	}
	return data
}

func (f *bootFixture) ping(success bool) {
	f.t.Helper()
	f.run(success, "ip", "netns", "exec", "nhop-client-inner", "ping", "-c", "1", "-W", "1", "198.51.100.7")
}

func (f *bootFixture) unitPath(name string) string {
	return filepath.Join("/run/systemd/system", name)
}

func (f *bootFixture) verifyNamespace() {
	f.t.Helper()
	path := fmt.Sprintf("/proc/%d/ns/net", f.child.Process.Pid)
	want := strings.TrimSpace(string(f.run(true, "ip", "netns", "exec", "nhop-client-inner", "readlink", "/proc/self/ns/net")))
	for end := time.Now().Add(time.Second); time.Now().Before(end); {
		got, err := os.Readlink(path)
		if err != nil {
			f.t.Fatal(err)
		}
		if got == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	f.t.Fatal("keeper process did not enter the verified private network namespace")
}
