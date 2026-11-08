package acceptance

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/control/client/session"
	"vpn-script/internal/control/client/systemd/install"
	"vpn-script/internal/core/validate"
	"vpn-script/internal/platform/safeio"
)

type installedService struct {
	managedService
	directory string
	override  []byte
	snapshot  map[string]string
}

func TestKernelInstalledServiceManagedRecovery(t *testing.T) {
	if os.Getenv("NHOP_CLIENT_INSTALLED_TEST") != "private-netns" {
		t.Skip("requires actual installed service with owned private network/NSS/DNS")
	}
	request, probe, initial, dns := managedContext(t)
	service := newInstalledService(t, probe, request)
	exerciseManaged(t, service.managedService, probe, initial, dns)
	if actual := captureInstalled(t, service.directory); !maps.Equal(actual, service.snapshot) {
		t.Fatal("running/restarting the installed service changed its immutable inputs")
	}
	t.Log("installed service: actual CLI, immutable runtime/profile snapshot, TCP/UDP/IPv6/DNS and retained guard recovery verified")
}

func newInstalledService(t *testing.T, probe *controller, request session.Request) installedService {
	t.Helper()
	profile, runtimes := installedInputs(t, request)
	args := []string{"service-install", "--profile", profile, "--runtimes", runtimes, "--ipv6", os.Getenv("NHOP_CLIENT_IPV6"), "--dns-ip", "198.18.0.2", "--dns-name", "dns.example.invalid"}
	if output, err := cliCommand(t, args...).Output(); err != nil {
		t.Fatalf("actual client installation failed: %v %s", err, output)
	}
	unit, err := safeio.Read(install.UnitPath, 65536)
	if err != nil {
		t.Fatal(err)
	}
	directory := installedDirectory(t, unit)
	value := installedService{managedService: managedService{name: install.Service, fragment: install.UnitPath, unit: unit,
		profile: filepath.Join(directory, "profile.json"), runtimes: filepath.Join(directory, "runtimes.json")}, directory: directory}
	value.snapshot = captureInstalled(t, directory)
	value.override = []byte("[Service]\n" + serviceIsolation(probe.command.Process.Pid))
	recordTestUnit(t, value.managedService)
	writeInstalledOverride(t, value)
	t.Cleanup(func() { value.cleanup(t) })
	for _, path := range []string{profile, runtimes} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("original installation profile/inventory removed before service start")
	serviceCommand(t, "daemon-reload")
	want := fmt.Sprintf("/proc/%d/ns/net", probe.command.Process.Pid)
	if actual := strings.TrimSpace(serviceCommand(t, "show", install.Service, "--value", "-p", "NetworkNamespacePath")); actual != want {
		t.Fatal("installed unit did not adopt the private network namespace", actual)
	}
	t.Logf("installed systemd command: %s", serviceCommand(t, "show", install.Service, "-p", "ExecStart", "-p", "BindPaths"))
	serviceCommand(t, "start", install.Service)
	return value
}

func installedInputs(t *testing.T, request session.Request) (string, string) {
	t.Helper()
	root := "/run/nhop-session-test/service inputs %$"
	request.Directory = filepath.Join(root, "active")
	request.Native.Path = filepath.Join(root, "xray")
	request.Frontend.Path = filepath.Join(root, "sing-box")
	request.Cronet.Path = filepath.Join(root, "libcronet.so")
	return cliInputs(t, request)
}

func installedDirectory(t *testing.T, unit []byte) string {
	t.Helper()
	fields := strings.Fields(strings.SplitN(string(unit), "\n", 2)[0])
	if len(fields) != 6 || !bytes.HasPrefix(unit, []byte("# nhop-client installation v1 ")) || !validate.Hash(fields[5]) {
		t.Fatal("unexpected installed unit identity")
	}
	directory := filepath.Join(install.Directory, "releases", fields[5])
	stored, err := safeio.Read(filepath.Join(directory, "unit"), 65536)
	if err != nil || !bytes.Equal(stored, unit) {
		t.Fatal("installed fragment differs from its generation", err)
	}
	want, err := safeio.FileHash(os.Getenv("NHOP_CLIENT_BINARY"))
	actual, other := safeio.FileHash(filepath.Join(directory, "nhop-client"))
	if err != nil || other != nil || actual != want {
		t.Fatal("installed client differs from the actual installer executable")
	}
	return directory
}

func writeInstalledOverride(t *testing.T, value installedService) {
	t.Helper()
	path := install.UnitPath + ".d"
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := safeio.Create(path+"/90-nhop-test.conf", value.override, 0600); err != nil {
		t.Fatal(err)
	}
	if err := safeio.Create("systemd-override.txt", value.override, 0600); err != nil {
		t.Fatal(err)
	}
	if err := safeio.NewJSON("installed-snapshot.json", value.snapshot); err != nil {
		t.Fatal(err)
	}
	recordInstalledInputs(t, value.directory)
}

func recordInstalledInputs(t *testing.T, directory string) {
	t.Helper()
	for name, path := range map[string]string{"profile": filepath.Join(directory, "profile.json"), "runtimes": filepath.Join(directory, "runtimes.json"),
		"release": filepath.Join(directory, "release.json"), "owner": filepath.Join(install.Directory, "owner.json")} {
		data, err := safeio.Read(path, 10_000_000)
		if err != nil {
			t.Fatal(err)
		}
		if err := safeio.Create("installed-"+name+".json", data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func installedFiles(directory string) map[string]os.FileMode {
	files := map[string]os.FileMode{filepath.Join(install.Directory, "owner.json"): 0600, filepath.Join(install.Directory, "install.lock"): 0600}
	for _, name := range []string{"nhop-client", "xray", "sing-box", "libcronet.so"} {
		files[filepath.Join(directory, name)] = 0755
	}
	for _, name := range []string{"profile.json", "runtimes.json", "release.json", "unit"} {
		files[filepath.Join(directory, name)] = 0600
	}
	return files
}

func captureInstalled(t *testing.T, directory string) map[string]string {
	t.Helper()
	for path, count := range map[string]int{install.Directory: 3, filepath.Join(install.Directory, "releases"): 1, directory: 8} {
		if err := safeio.CheckAccess(path, os.ModeDir|0700, 0, 0); err != nil {
			t.Fatal(err)
		}
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != count {
			t.Fatal("unexpected installed directory contents", path, err)
		}
	}
	result := map[string]string{}
	for path, mode := range installedFiles(directory) {
		if err := safeio.CheckAccess(path, mode, 0, 0); err != nil {
			t.Fatal(err)
		}
		digest, err := safeio.FileHash(path)
		if err != nil {
			t.Fatal(err)
		}
		result[path] = digest
	}
	return result
}

func (s installedService) cleanup(t *testing.T) {
	t.Helper()
	actual, err := safeio.Read(install.UnitPath, 65536)
	override, other := safeio.Read(install.UnitPath+".d/90-nhop-test.conf", 65536)
	if err != nil || other != nil || !bytes.Equal(actual, s.unit) || !bytes.Equal(override, s.override) {
		t.Error("refusing cleanup of changed installed service context", err, other)
		return
	}
	serviceCommand(t, "stop", install.Service)
	cliCall(t, "stop")
	serviceJournal(t, install.Service)
	if !maps.Equal(captureInstalled(t, s.directory), s.snapshot) {
		t.Error("refusing to remove modified installed snapshot")
		return
	}
	for _, path := range []string{install.UnitPath + ".d/90-nhop-test.conf", install.UnitPath + ".d"} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	serviceCommand(t, "daemon-reload")
	if output, err := cliCommand(t, "service-remove").Output(); err != nil {
		t.Fatalf("cannot unregister owned service: %v %s", err, output)
	}
	removeInstalledFiles(t, s)
	t.Log("installed fixture removed: unit, generation and isolation drop-in absent")
}

func removeInstalledFiles(t *testing.T, s installedService) {
	t.Helper()
	for path := range s.snapshot {
		if err := os.Remove(path); err != nil {
			t.Fatal("cannot remove verified fixture snapshot", err)
		}
	}
	for _, path := range []string{s.directory, filepath.Join(install.Directory, "releases"), install.Directory} {
		if err := os.Remove(path); err != nil {
			t.Fatal("refusing recursive cleanup of unexpected fixture files", err)
		}
	}
}
