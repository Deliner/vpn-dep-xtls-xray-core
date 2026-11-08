package service

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestUnitKeepsLiteralArgumentsAndPersistentRevocationPolicy(t *testing.T) {
	raw, err := Unit("/var/lib/fleet build/agent%h", "/var/lib/fleet $TOKEN/state", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`ExecStart=:"/var/lib/fleet build/agent%%h" "agent-run"`, `"--state" "/var/lib/fleet $TOKEN/state"`,
		fmt.Sprintf("RestartPreventExitStatus=%d\n", AccessDeniedExit), "TimeoutStopSec=120s\n", "WantedBy=multi-user.target\n"} {
		if !strings.Contains(string(raw), fragment) {
			t.Fatalf("unit lost its literal arguments or persistence policy: %q", fragment)
		}
	}
	if strings.Contains(string(raw), "ExecStop=") || strings.Contains(string(raw), "Requires=vpn-nhop") || strings.Contains(string(raw), "RuntimeDirectory=") {
		t.Fatal("agent lifetime was coupled to VPN or durable journal removal")
	}
}

func TestUnitRejectsHiddenTransientAndAmbiguousPaths(t *testing.T) {
	for _, state := range []string{"relative", "/", "/var/lib/a/../b", "/root/state", "/home/a/state", "/tmp/state", "/var/tmp/state", "/run/state", "/var/lib/a\nExecStart=bad"} {
		if _, err := Unit("/opt/fleet/agent", state, time.Second); err == nil {
			t.Errorf("accepted unsafe service state path %q", state)
		}
	}
	for _, interval := range []time.Duration{0, time.Millisecond, time.Hour + time.Second} {
		if _, err := Unit("/opt/fleet/agent", "/var/lib/fleet-node", interval); err == nil {
			t.Fatal("invalid service polling interval accepted")
		}
	}
}
