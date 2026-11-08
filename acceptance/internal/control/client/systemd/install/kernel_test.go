package install

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/platform/safeio"
)

func TestKernelClientServiceInstallation(t *testing.T) {
	if os.Getenv("NHOP_CLIENT_INSTALL_TEST") != "private-netns" {
		t.Skip("requires owned Linux systemd with isolated controller state")
	}
	kernelIsolation(t)
	t.Cleanup(func() { kernelCleanup(t) })
	options := kernelInputs(t)
	args := []string{"service-install", "--profile", options.Profile, "--runtimes", options.Runtimes}
	kernelCLI(t, true, args...)
	first, unit := kernelInstalled(t)
	kernelCLI(t, true, args...)
	if next, data := kernelInstalled(t); next != first || !bytes.Equal(data, unit) {
		t.Fatal("real idempotent install changed the generation")
	}
	owner, err := host.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	kernelCLI(t, false, args...)
	owner.Close()
	kernelCLI(t, true, append(args, "--mtu", "1400")...)
	second, _ := kernelInstalled(t)
	if second.Generation == first.Generation {
		t.Fatal("updated policy did not create a new installed generation")
	}
	kernelCLI(t, true, args...)
	if restored, data := kernelInstalled(t); restored != first || !bytes.Equal(data, unit) {
		t.Fatal("real systemd rollback changed the original snapshot")
	}
	kernelInterrupted(t, args, second)
	kernelCLI(t, true, "service-remove")
	if _, err := os.Lstat(UnitPath); !os.IsNotExist(err) {
		t.Fatal("unit survived successful unregister", err)
	}
	t.Logf("installation verified: initial/idempotent/update/rollback/interrupted reload/remove; generations %s %s retained", first.Generation, second.Generation)
}

func kernelIsolation(t *testing.T) {
	t.Helper()
	if err := host.LinuxRoot(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"net", "mnt"} {
		current, err := os.Readlink("/proc/self/ns/" + name)
		original, other := os.Readlink("/proc/1/ns/" + name)
		if err != nil || other != nil || current == original {
			t.Fatal("installation fixture requires private namespaces", name)
		}
	}
	for _, path := range []string{Directory, UnitPath, UnitPath + ".d"} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("refusing to touch pre-existing installation", path, err)
		}
	}
}

func kernelInputs(t *testing.T) Options {
	t.Helper()
	root := filepath.Join(host.Directory, "installation-inputs")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	value := Options{Profile: filepath.Join(root, "profile.json"), Runtimes: filepath.Join(root, "runtimes.json")}
	if err := safeio.NewJSON(value.Profile, importedProfile(t)); err != nil {
		t.Fatal(err)
	}
	var artifacts []session.Artifact
	for _, name := range []string{"xray", "sing-box", "libcronet.so"} {
		path := filepath.Join(os.Getenv("NHOP_CLIENT_RUNTIME"), name)
		digest, err := safeio.FileHash(path)
		if err != nil {
			t.Fatal(err)
		}
		artifacts = append(artifacts, session.Artifact{Path: path, SHA256: digest})
	}
	inventory := controller.Runtimes{SchemaVersion: 1, Xray: artifacts[0], SingBox: artifacts[1], Cronet: artifacts[2]}
	if err := safeio.NewJSON(value.Runtimes, inventory); err != nil {
		t.Fatal(err)
	}
	return value
}

func kernelCLI(t *testing.T, success bool, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Getenv("NHOP_CLIENT_BINARY"), args...)
	data, err := command.CombinedOutput()
	if (err == nil) != success {
		t.Fatalf("unexpected client operation %s result: %v %s", args[0], err, data)
	}
	t.Logf("actual CLI %s success=%t: %s", args[0], success, data)
}

func kernelInstalled(t *testing.T) (Receipt, []byte) {
	t.Helper()
	tr, err := openTransaction()
	if err != nil {
		t.Fatal(err)
	}
	defer tr.store.lock.Close()
	unit, err := tr.store.current()
	if err != nil || len(unit) == 0 {
		t.Fatal("cannot verify installed unit and immutable inputs", err)
	}
	line := strings.SplitN(string(unit), "\n", 2)[0]
	id := strings.TrimPrefix(line, marker+tr.store.owner.Token+" ")
	r := Receipt{Generation: id, Unit: UnitPath, Directory: filepath.Join(Directory, "releases", id)}
	want, err := safeio.FileHash(os.Getenv("NHOP_CLIENT_BINARY"))
	got, other := safeio.FileHash(filepath.Join(r.Directory, "nhop-client"))
	if err != nil || other != nil || want != got {
		t.Fatal("installed executable differs from the actual running CLI")
	}
	if err := tr.inactive(context.Background(), "present"); err != nil {
		t.Fatal(err)
	}
	data, err := exec.Command(filepath.Join(r.Directory, "nhop-client"), "--help").CombinedOutput()
	if err != nil || !bytes.Contains(data, []byte("service-install")) {
		t.Fatal("copied CLI cannot execute", err)
	}
	return r, unit
}

func kernelInterrupted(t *testing.T, args []string, desired Receipt) {
	t.Helper()
	kernelPinUnit(t)
	tr, err := openTransaction()
	if err != nil {
		t.Fatal(err)
	}
	previous, err := tr.store.current()
	if err != nil {
		t.Fatal(err)
	}
	unit, err := tr.store.generation(desired.Generation)
	if err != nil || tr.replace(previous, unit) != nil {
		t.Fatal("cannot simulate a committed update interrupted before daemon-reload", err)
	}
	tr.store.lock.Close()
	manager, err := systemManager()
	if err != nil {
		t.Fatal(err)
	}
	data, err := manager(context.Background(), "show", "--value", "-p", "NeedDaemonReload", Service)
	if err != nil || strings.TrimSpace(string(data)) != "yes" {
		t.Fatal("systemd did not observe the pending interrupted publication", err, string(data))
	}
	kernelCLI(t, true, append(args, "--mtu", "1400")...)
	data, err = manager(context.Background(), "show", "--value", "-p", "NeedDaemonReload", Service)
	if err != nil || strings.TrimSpace(string(data)) != "no" {
		t.Fatal("retry did not reconcile the committed generation", err, string(data))
	}
}

// systemd may garbage-collect an inactive unreferenced service and load its new
// disk unit on the next query. An inert target keeps the old service referenced
// without starting it, making the interrupted-reload scenario deterministic.
func kernelPinUnit(t *testing.T) {
	t.Helper()
	name := fmt.Sprintf("nhop-client-install-pin-%d.target", os.Getpid())
	path := filepath.Join("/run/systemd/system", name)
	unit := []byte("[Unit]\nDescription=Owned client install regression reference\nAfter=" + Service + "\n")
	if err := safeio.Create(path, unit, 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := systemManager()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { kernelUnpin(t, manager, name, path, unit) })
	for _, args := range [][]string{{"daemon-reload"}, {"start", name}} {
		if _, err := manager(context.Background(), args...); err != nil {
			t.Fatal("cannot retain inactive unit in the real manager", err)
		}
	}
}

func kernelUnpin(t *testing.T, manager manager, name, path string, expected []byte) {
	t.Helper()
	actual, err := safeio.Read(path, 4096)
	if err != nil || !bytes.Equal(actual, expected) {
		t.Error("refusing cleanup of changed fixture target", err)
		return
	}
	if _, err := manager(context.Background(), "stop", name); err != nil {
		t.Error("cannot stop owned fixture target", err)
		return
	}
	if err := os.Remove(path); err != nil {
		t.Error(err)
		return
	}
	if _, err := manager(context.Background(), "daemon-reload"); err != nil {
		t.Error(err)
	}
}

func kernelCleanup(t *testing.T) {
	t.Helper()
	tr, err := openTransaction()
	if err != nil {
		t.Error("cannot inspect installation fixture for cleanup", err)
		return
	}
	defer tr.store.lock.Close()
	if err := tr.remove(context.Background()); err != nil {
		t.Error("refusing cleanup of uncertain installation state", err)
		return
	}
	if err := removeFixtureGenerations(tr.store); err != nil {
		t.Error("refusing cleanup of unverified generation", err)
		return
	}
	for _, name := range []string{"releases", "owner.json", "install.lock", "."} {
		if err := os.Remove(filepath.Join(Directory, name)); err != nil {
			t.Error("cannot remove empty owned installation fixture", err)
			return
		}
	}
}

func removeFixtureGenerations(s *store) error {
	entries, err := os.ReadDir(filepath.Join(s.base, "releases"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if _, err := s.generation(entry.Name()); err != nil {
			return err
		}
	}
	for _, entry := range entries {
		directory := filepath.Join(s.base, "releases", entry.Name())
		for _, name := range []string{"nhop-client", "xray", "sing-box", "libcronet.so", "profile.json", "runtimes.json", "release.json", "unit", "."} {
			if err := os.Remove(filepath.Join(directory, name)); err != nil {
				return fmt.Errorf("fixture generation cleanup: %w", err)
			}
		}
	}
	return nil
}
