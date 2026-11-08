package acceptance

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/control/client/systemd"
	"vpn-script/internal/platform/safeio"
)

type managedService struct {
	name, fragment, profile, runtimes string
	unit                              []byte
}

func newManagedService(t *testing.T, probe *controller, request session.Request) managedService {
	t.Helper()
	root := "/run/nhop-session-test/service inputs %$"
	request.Directory = filepath.Join(root, "active")
	request.Native.Path = filepath.Join(root, "xray")
	request.Frontend.Path = filepath.Join(root, "sing-box")
	request.Cronet.Path = filepath.Join(root, "libcronet.so")
	profile, runtimes := cliInputs(t, request)
	policy := network.Defaults(0)
	policy.IPv6, policy.DNSIP, policy.DNSName = os.Getenv("NHOP_CLIENT_IPV6"), "198.18.0.2", "dns.example.invalid"
	unit, err := systemd.Unit(systemd.Options{Executable: os.Getenv("NHOP_CLIENT_SYSTEMD_BINARY"), Profile: profile, Runtimes: runtimes, Network: policy})
	if err != nil {
		t.Fatal(err)
	}
	unit = append(unit, []byte(serviceIsolation(probe.command.Process.Pid))...)
	name := os.Getenv("NHOP_CLIENT_SYSTEMD_UNIT")
	if !strings.HasPrefix(name, "nhop-client-acceptance-") || filepath.Base(name) != name || !strings.HasSuffix(name, ".service") {
		t.Fatal("missing owned acceptance service name")
	}
	value := managedService{name: name, fragment: "/run/systemd/system/" + name, profile: profile, runtimes: runtimes, unit: unit}
	publishTestUnit(t, value)
	t.Cleanup(func() { value.cleanup(t) })
	serviceCommand(t, "daemon-reload")
	t.Logf("parsed systemd command: %s", serviceCommand(t, "show", name, "-p", "ExecStart", "-p", "BindPaths"))
	serviceCommand(t, "start", name)
	return value
}

func serviceIsolation(probePID int) string {
	root := os.Getenv("NHOP_CLIENT_SYSTEMD_ROOT")
	value := fmt.Sprintf("\n# Test-only private namespace context; production settings above are unchanged.\nNetworkNamespacePath=/proc/%d/ns/net\n", probePID)
	for _, pair := range [][2]string{{"session/etc", "/etc"}, {"control", "/run/nhop-client"}, {"session", "/run/nhop-session-test"}, {"resolve", "/run/systemd/resolve"}, {"netif", "/run/systemd/netif"}} {
		value += "BindPaths=" + root + "/" + pair[0] + ":" + pair[1] + "\n"
	}
	value += "Environment=DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/nhop-session-test/bus GORACE=atexit_sleep_ms=0\nRuntimeMaxSec=90s\n"
	return value
}

func publishTestUnit(t *testing.T, value managedService) {
	t.Helper()
	if err := safeio.Create("systemd-unit.txt", value.unit, 0600); err != nil {
		t.Fatal(err)
	}
	// The outer shell can clean up this exact unit even if the Go test times out.
	if err := safeio.Create(value.fragment, value.unit, 0600); err != nil {
		t.Fatal(err)
	}
	digest, err := safeio.FileHash(value.fragment)
	if err == nil {
		err = safeio.Create("systemd-unit.sha256", []byte(digest+"  "+value.fragment+"\n"), 0600)
	}
	if err != nil {
		os.Remove(value.fragment)
		t.Fatal(err)
	}
}

func manager(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "systemctl", args...)
	command.Env = managerEnvironment()
	return command.CombinedOutput()
}

func managerEnvironment() []string {
	var environment []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "DBUS_SYSTEM_BUS_ADDRESS=") {
			environment = append(environment, value)
		}
	}
	return environment
}

func serviceCommand(t *testing.T, args ...string) string {
	t.Helper()
	data, err := manager(args...)
	if err != nil {
		t.Fatalf("owned systemd operation %v failed: %v %s", args, err, data)
	}
	return string(data)
}

func verifyManagedNamespace(t *testing.T, probe *controller, status session.Status) {
	t.Helper()
	expected, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/net", probe.command.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	for _, pid := range []int{status.NativePID, status.FrontendPID} {
		actual, err := os.Readlink("/proc/" + strconv.Itoa(pid) + "/ns/net")
		if err != nil || actual != expected {
			t.Fatal("managed engine escaped the fixture network namespace", pid, err)
		}
	}
}

func (s managedService) cleanup(t *testing.T) {
	t.Helper()
	actual, err := safeio.Read(s.fragment, 1<<20)
	if err != nil || !bytes.Equal(actual, s.unit) {
		t.Error("refusing cleanup of changed unit definition", err)
		return
	}
	if data, err := manager("stop", s.name); err != nil {
		t.Error("cannot stop owned unit", err, string(data))
		return
	}
	cliCall(t, "stop")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	logs, err := exec.CommandContext(ctx, "journalctl", "--no-pager", "-u", s.name).CombinedOutput()
	t.Logf("systemd journal: %s", logs)
	if err != nil {
		t.Error("cannot collect service journal", err)
	}
	if err := os.Remove(s.fragment); err != nil {
		t.Error(err)
	}
	if data, err := manager("daemon-reload"); err != nil {
		t.Error("cannot unload owned unit", err, string(data))
	}
}
