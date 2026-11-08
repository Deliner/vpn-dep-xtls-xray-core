package canary

import (
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/platform/safeio"
)

func (f *fixture) machines(t *testing.T) {
	t.Helper()
	boots := map[string]bool{}
	for _, node := range nodes {
		host := strings.TrimSpace(string(f.guest(t, node, "hostname")))
		if host != "lima-"+node {
			t.Fatal("unexpected VM identity", host)
		}
		boot := strings.TrimSpace(string(f.guest(t, node, "cat", "/proc/sys/kernel/random/boot_id")))
		address := f.address(t, node)
		if boots[boot] || boot == "" {
			t.Fatal("three independently booted kernels are required")
		}
		boots[boot], f.addresses[node] = true, address
		f.report.Machines[node] = machine{Address: address, BootID: boot, Kernel: strings.TrimSpace(string(f.guest(t, node, "uname", "-a")))}
		f.guest(t, node, "test", "!", "-e", f.remote)
		f.guest(t, node, "install", "-d", "-m", "0700", "--", f.remote)
		for _, name := range []string{"nhop-fleet", "canary.test"} {
			f.put(t, node, filepath.Join(f.root, name), name, "0700")
			expected, err := safeio.FileHash(filepath.Join(f.root, name))
			require(t, err)
			if strings.Fields(string(f.guest(t, node, "sha256sum", f.remote+"/"+name)))[0] != expected {
				t.Fatal("guest binary digest changed during transfer")
			}
		}
		f.guestTest(t, node, "TestFleetGuestGuard")
		t.Cleanup(func() { f.cleanMachine(t, node) })
	}
}

func (f *fixture) address(t *testing.T, node string) string {
	t.Helper()
	type info struct {
		Local string `json:"local"`
	}
	type device struct {
		Addresses []info `json:"addr_info"`
	}
	devices := decode[[]device](t, f.guest(t, node, "ip", "-j", "-4", "address", "show"))
	for _, device := range devices {
		for _, address := range device.Addresses {
			if strings.HasPrefix(address.Local, "192.168.124.") {
				for _, prior := range f.addresses {
					if prior == address.Local {
						t.Fatal("Fleet VM addresses must be distinct")
					}
				}
				return address.Local
			}
		}
	}
	t.Fatal("VM is not attached to the owned canary network")
	return ""
}

func (f *fixture) cleanMachine(t *testing.T, node string) {
	t.Helper()
	raw := f.guest(t, node, "journalctl", "--no-pager", "-u", "nhop-fleet-agent.service", "-u", "vpn-nhop.service", "-u", controllerUnit)
	require(t, safeio.Atomic(filepath.Join(f.root, node+"-journal.log"), raw, 0600))
	f.guestTest(t, node, "TestFleetGuestClean")
	f.command(t, "shell", "--workdir", "/home/nhop", node, "rmdir", "--", "/home/nhop/"+filepath.Base(f.remote))
}

func (f *fixture) save(t *testing.T) {
	t.Helper()
	if !t.Failed() {
		for _, node := range nodes {
			f.guest(t, node, "rm", "-rf", "--", f.remote)
		}
	}
	f.report.Status = "passed"
	f.report.CleanupSucceeded = !t.Failed()
	if t.Failed() {
		f.report.Status = "failed"
	}
	if err := safeio.JSON(filepath.Join(f.root, "canary.json"), f.report); err != nil {
		t.Error(err)
	}
}
