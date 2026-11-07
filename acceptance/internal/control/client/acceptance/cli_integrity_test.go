package acceptance

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	clientcontrol "vpn-script/internal/control/client/controller"
	"vpn-script/internal/control/client/host"
	"vpn-script/internal/platform/safeio"
)

func refuseBadRuntime(t *testing.T, probe *controller, profile, runtimes string) {
	t.Helper()
	var inventory clientcontrol.Runtimes
	if err := safeio.LoadJSON(runtimes, &inventory); err != nil {
		t.Fatal(err)
	}
	inventory.Xray.SHA256 = strings.Repeat("0", 64)
	path := filepath.Join(filepath.Dir(profile), "tampered-runtimes.json")
	if err := safeio.NewJSON(path, inventory); err != nil {
		t.Fatal(err)
	}
	before := sessionNames(t)
	journal, err := os.ReadFile(filepath.Join(host.Directory, "current.json"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	routes, dns := probe.call(t, "routes").Routes, probe.call(t, "dns").DNS
	if data, err := cliCommand(t, "start", "--profile", profile, "--runtimes", path).Output(); err == nil {
		t.Fatalf("changed runtime was executed: %s", data)
	}
	if sessionNames(t) != before || probe.call(t, "routes").Routes != routes || probe.call(t, "dns").DNS != dns {
		t.Fatal("failed runtime snapshot changed session inventory or host networking")
	}
	after, err := os.ReadFile(filepath.Join(host.Directory, "current.json"))
	if err != nil && !os.IsNotExist(err) || string(after) != string(journal) {
		t.Fatal("unstarted session lost the prior journal")
	}
	checkTraffic(t, probe, true)
}

func sessionNames(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(host.Directory, "sessions"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return strings.Join(names, ",")
}
