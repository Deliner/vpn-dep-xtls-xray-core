package masque

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"vpn-script/internal/control/integration/evidence/installation"
	"vpn-script/internal/platform/process"
)

func requireIsolation(t *testing.T) {
	t.Helper()
	if err := installation.OwnedHost(); err != nil || os.Getenv("NHOP_MASQUE_NETEM_TEST") != "private-netns" {
		t.Fatalf("MASQUE faults require an owned Linux test host and private namespaces: %v", err)
	}
	for _, name := range []string{"net", "mnt", "pid"} {
		actual, err := os.Readlink("/proc/self/ns/" + name)
		original := os.Getenv("NHOP_MASQUE_HOST_" + name)
		if err != nil || original == "" || actual == original {
			t.Fatalf("missing %s namespace isolation: %s / %s / %v", name, actual, original, err)
		}
	}
}

func shell(t *testing.T, namespace string, arguments ...string) string {
	t.Helper()
	if namespace != "" {
		arguments = append([]string{"ip", "netns", "exec", namespace}, arguments...)
	}
	result, err := process.Run(context.Background(), 5*time.Second, arguments...)
	if err != nil {
		t.Fatalf("%v: %v: %s", arguments, err, result.Output)
	}
	return string(result.Output)
}

func shape(t *testing.T, state impairment) {
	t.Helper()
	for _, namespace := range []string{"", exitNamespace} {
		shell(t, namespace, "ip", "link", "set", "nhop-link", "mtu", strconv.Itoa(state.MTU))
		arguments := []string{"tc", "qdisc", "replace", "dev", "nhop-link", "root", "handle", "10:", "netem", "limit", "4096"}
		shell(t, namespace, append(arguments, state.Netem...)...)
	}
}

func boundedEnvironment(t *testing.T, name string, fallback, maximum int) int {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < 1 || parsed > maximum {
		t.Fatal(fmt.Sprintf("%s must be 1..%d", name, maximum))
	}
	return parsed
}
