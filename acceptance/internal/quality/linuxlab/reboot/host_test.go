package reboot

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

func (f *fixture) attempt(timeout time.Duration, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := process.Command(ctx, f.lima, args...)
	return command.CombinedOutput()
}

func (f *fixture) command(t *testing.T, args ...string) []byte {
	t.Helper()
	raw, err := f.attempt(90*time.Second, args...)
	if err != nil {
		path := filepath.Join(f.root, fmt.Sprintf("command-failure-%d.log", time.Now().UnixNano()))
		require(t, safeio.Atomic(path, raw, 0600))
		t.Fatalf("owned reboot VM command failed: %v; private output %s", err, path)
	}
	return raw
}

func (f *fixture) remote(t *testing.T, role string, args ...string) []byte {
	t.Helper()
	if role != "boot-client" && role != "boot-exit" {
		t.Fatal("unknown reboot VM")
	}
	return f.command(t, append([]string{"shell", "--workdir", "/home/nhop", role, "sudo", "--"}, args...)...)
}

func (f *fixture) guest(t *testing.T, role, name string) []byte {
	t.Helper()
	raw := f.remote(t, role, "env", "NHOP_REBOOT_GUEST=owned-vm", contract.Root+"/reboot.test", "-test.v", "-test.run=^"+name+"$", "-test.timeout=80s")
	path := filepath.Join(f.root, fmt.Sprintf("%s-%s-%d.log", role, name, time.Now().UnixNano()))
	require(t, safeio.Atomic(path, raw, 0600))
	return raw
}

func (f *fixture) put(t *testing.T, role, local, name, mode string) {
	t.Helper()
	stage := "/home/nhop/runs/reboot-" + filepath.Base(f.root)
	f.command(t, "shell", "--workdir", "/home/nhop", role, "mkdir", "-p", "--", stage)
	temporary := stage + "/" + strings.ReplaceAll(name, "/", "-")
	f.command(t, "copy", local, role+":"+temporary)
	f.remote(t, role, "install", "-m", mode, "--", temporary, contract.Root+"/"+name)
	f.command(t, "shell", "--workdir", "/home/nhop", role, "rm", "--", temporary)
	expected, err := safeio.FileHash(local)
	require(t, err)
	if strings.Fields(string(f.remote(t, role, "sha256sum", contract.Root+"/"+name)))[0] != expected {
		t.Fatal("guest file digest changed during transfer")
	}
}

func (f *fixture) putJSON(t *testing.T, role, name string, value any) {
	t.Helper()
	local := filepath.Join(f.root, role+"-"+strings.ReplaceAll(name, "/", "-"))
	require(t, safeio.JSON(local, value))
	f.put(t, role, local, name, "0600")
}

func (f *fixture) save(t *testing.T, role, remote, name string) []byte {
	t.Helper()
	raw := f.remote(t, role, "cat", "--", remote)
	require(t, safeio.Atomic(filepath.Join(f.root, role+"-"+name), raw, 0600))
	return raw
}
