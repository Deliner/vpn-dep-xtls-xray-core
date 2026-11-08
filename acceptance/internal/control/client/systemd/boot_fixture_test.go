package systemd_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/systemd"
	"vpn-script/internal/control/client/systemd/manager"
	"vpn-script/internal/platform/safeio"
)

func bootSetup(t *testing.T) *bootFixture {
	t.Helper()
	if os.Getenv("NHOP_CLIENT_BOOT_ORDER_TEST") != "private-netns" {
		t.Skip("requires owned Linux systemd and private network/mount namespaces")
	}
	bootIsolation(t)
	root := fmt.Sprintf("/opt/nhop-boot-order-%d", os.Getpid())
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	f := &bootFixture{t: t, root: root, binary: root + "/nhop-client", state: root + "/control",
		guard: fmt.Sprintf("nhop-boot-order-%d.service", os.Getpid()), consumer: fmt.Sprintf("nhop-boot-consumer-%d.service", os.Getpid()), files: map[string][]byte{}}
	t.Cleanup(f.close)
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	f.prepare()
	return f
}

func bootIsolation(t *testing.T) {
	t.Helper()
	if err := host.LinuxRoot(); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"net", "mnt"} {
		a, err := os.Readlink("/proc/self/ns/" + kind)
		b, other := os.Readlink("/proc/1/ns/" + kind)
		if err != nil || other != nil || a == b {
			t.Fatal("boot order test requires private namespaces", kind)
		}
	}
}

func (f *bootFixture) prepare() {
	f.t.Helper()
	data, err := os.ReadFile(os.Getenv("NHOP_CLIENT_BINARY"))
	if err != nil {
		f.t.Fatal(err)
	}
	if err := safeio.Create(f.binary, data, 0755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Mkdir(f.state, 0711); err != nil {
		f.t.Fatal(err)
	}
	if err := os.Chmod(f.state, 0711); err != nil {
		f.t.Fatal(err)
	}
	f.run(true, "mount", "--bind", f.state, host.Directory)
	f.mounted = true
	f.child = exec.Command("ip", "netns", "exec", "nhop-client-inner", "sleep", "120")
	if err := f.child.Start(); err != nil {
		f.t.Fatal(err)
	}
	f.verifyNamespace()
	f.command, err = manager.New()
	if err != nil {
		f.t.Fatal(err)
	}
	f.unownedUnitsAbsent()
	f.writeGuard()
}

func (f *bootFixture) unownedUnitsAbsent() {
	f.t.Helper()
	for _, unit := range []string{f.guard, f.consumer} {
		if _, err := os.Lstat(f.unitPath(unit)); !os.IsNotExist(err) {
			f.t.Fatal("test unit path already exists", unit, err)
		}
		data, _ := f.command(f.t.Context(), "show", "--value", "--property=LoadState", "--", unit)
		if strings.TrimSpace(string(data)) != "not-found" {
			f.t.Fatal("test unit name is already registered", unit, string(data))
		}
	}
}

func (f *bootFixture) context() string {
	return fmt.Sprintf("NetworkNamespacePath=/proc/%d/ns/net\nBindPaths=%s:%s\n", f.child.Process.Pid, f.state, host.Directory)
}

func (f *bootFixture) writeGuard() {
	f.t.Helper()
	data, err := systemd.BootUnit(f.binary)
	if err != nil {
		f.t.Fatal(err)
	}
	f.publish(f.guard, append(data, []byte(f.context())...))
}

func (f *bootFixture) writeConsumer(strong bool) {
	f.t.Helper()
	unit := strings.ReplaceAll(systemd.BootRequirement, systemd.BootService, f.guard)
	if !strong {
		unit = strings.ReplaceAll(unit, "Requires=", "Wants=")
	}
	unit += "[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=/usr/bin/touch " + filepath.Join(f.root, "reached") + "\n" + f.context()
	f.publish(f.consumer, []byte(unit))
	f.control(true, "daemon-reload")
}

func (f *bootFixture) publish(name string, data []byte) {
	f.t.Helper()
	path := f.unitPath(name)
	var err error
	if expected, found := f.files[path]; found {
		actual, readErr := safeio.Read(path, 65536)
		if readErr != nil || !bytes.Equal(actual, expected) {
			f.t.Fatal("test unit changed before replacement", readErr)
		}
		err = safeio.Atomic(path, data, 0644)
	} else {
		err = safeio.Create(path, data, 0644)
	}
	if err != nil {
		f.t.Fatal(err)
	}
	f.files[path] = data
}
