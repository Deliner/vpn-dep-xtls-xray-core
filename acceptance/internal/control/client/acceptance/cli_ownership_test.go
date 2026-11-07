package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	clientcontrol "vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/host"
	"vpn-script/internal/control/client/session"
	"vpn-script/internal/platform/safeio"
)

func refuseDuplicate(t *testing.T, profile, runtimes string, started clientcontrol.Diagnostic) {
	t.Helper()
	data, err := cliCommand(t, "start", "--profile", profile, "--runtimes", runtimes).Output()
	if err == nil {
		t.Fatalf("second controller acquired active state: %s", data)
	}
	actual := cliCall(t, "diagnostics")
	if actual.Controller != "active" || actual.Status.NativePID != started.Status.NativePID || actual.Directory != started.Directory {
		t.Fatal("second controller changed the active session")
	}
	verifyPreviousSnapshots(t, started.Directory)
}

func verifyPreviousSnapshots(t *testing.T, active string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(host.Directory, "sessions"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		directory := filepath.Join(host.Directory, "sessions", entry.Name())
		if directory == active {
			continue
		}
		for _, name := range []string{"native", "frontend", "libcronet.so"} {
			if _, err := os.Lstat(filepath.Join(directory, name)); !os.IsNotExist(err) {
				t.Fatal("stopped runtime snapshot was not pruned", name, err)
			}
		}
		if err := safeio.CheckAccess(filepath.Join(directory, "request.json"), 0600, 0, 0); err != nil {
			t.Fatal("cleanup lost the private session journal", err)
		}
	}
}

func refuseChangedLease(t *testing.T, probe *controller, directory string) {
	t.Helper()
	path := filepath.Join(directory, "status.json")
	var original session.Status
	if err := safeio.LoadJSON(path, &original); err != nil {
		t.Fatal(err)
	}
	defer func() { safeio.JSON(path, original) }()
	changed := original
	changed.Guard.Token = "nhop:" + strings.Repeat("f", 32)
	if err := safeio.JSON(path, changed); err != nil {
		t.Fatal(err)
	}
	before := probe.call(t, "routes").Routes
	if data, err := cliCommand(t, "stop").Output(); err == nil {
		t.Fatalf("recovery accepted an unmatched firewall lease: %s", data)
	}
	if probe.call(t, "routes").Routes != before {
		t.Fatal("rejected recovery mutated routing rules")
	}
	checkTraffic(t, probe, false)
}
