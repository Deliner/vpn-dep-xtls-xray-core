package resolver

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func TestKernelResolvedPrivateLinkLifecycle(t *testing.T) {
	if os.Getenv("NHOP_CLIENT_RESOLVED_TEST") != "private-netns" {
		t.Skip("requires private Linux mount/network namespaces and private D-Bus")
	}
	checkKernelIsolation(t)
	kernelResolved(t)
	link, err := net.InterfaceByName("nhopdns")
	if err != nil {
		t.Fatal(err)
	}
	m, err := New()
	if err != nil {
		t.Fatal(err)
	}
	before := resolvedState(t)
	lease := Link{Name: link.Name, Index: link.Index}
	if err := m.ConfigureNewLink(t.Context(), lease); err != nil {
		t.Fatal(err)
	}
	checkResolvedPolicy(t)
	if err := m.Revert(t.Context(), lease); err != nil {
		t.Fatal(err)
	}
	if after := resolvedState(t); after != before {
		t.Fatalf("system DNS was not restored:\nbefore: %s\nafter: %s", before, after)
	}
}

func checkKernelIsolation(t *testing.T) {
	t.Helper()
	hostname, err := os.Hostname()
	if err != nil || hostname != "lima-payload" || os.Geteuid() != 0 {
		t.Fatal("resolver kernel test requires owned root Linux guest")
	}
	for _, namespace := range []string{"net", "mnt"} {
		self, err := os.Readlink("/proc/self/ns/" + namespace)
		initial, initialErr := os.Readlink("/proc/1/ns/" + namespace)
		if err != nil || initialErr != nil || self == initial {
			t.Fatal("resolver test requires private namespaces")
		}
	}
}

func resolvedOutput(t *testing.T, args ...string) string {
	t.Helper()
	result, err := process.Run(t.Context(), 3*time.Second, append([]string{"resolvectl"}, args...)...)
	if err != nil || !result.Success {
		t.Fatalf("resolvectl %v: %v: %s", args, err, result.Output)
	}
	return string(result.Output)
}

func resolvedState(t *testing.T) string {
	t.Helper()
	return resolvedOutput(t, "dns") + resolvedOutput(t, "domain") + resolvedOutput(t, "default-route")
}

func checkResolvedPolicy(t *testing.T) {
	t.Helper()
	for _, item := range []struct{ command, value string }{{"dns", "172.30.255.2"}, {"domain", "~."},
		{"default-route", "yes"}, {"llmnr", "no"}, {"mdns", "no"}, {"dnsovertls", "no"}} {
		output := resolvedOutput(t, item.command, "nhopdns")
		if !strings.Contains(output, "nhopdns") || !strings.Contains(output, item.value) {
			t.Fatalf("private DNS policy not applied: %s: %s", item.command, output)
		}
		t.Log(strings.TrimSpace(output))
	}
}
