package systemd_test

import (
	"strings"
	"testing"

	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/systemd"
)

func TestServicePlanQuotesLiteralArgumentsAndPreservesOwnership(t *testing.T) {
	value := systemd.Options{Executable: `/opt/client/nhop`, Profile: `/etc/nhop/a %n $HOME "b"\c.json`, Runtimes: `/etc/nhop/runtimes.json`, Network: network.Defaults(0)}
	data, err := systemd.Unit(value)
	if err != nil {
		t.Fatal(err)
	}
	unit := string(data)
	if !strings.Contains(unit, `"/etc/nhop/a %%n $HOME \"b\"\\c.json"`) || !strings.Contains(unit, `ExecStart=:"/opt/client/nhop"`) {
		t.Fatal("arguments may be expanded or split by systemd", unit)
	}
	for _, forbidden := range []string{"\nExecStop=", "\nRuntimeDirectory=", "\n[Install]"} {
		if strings.Contains(unit, forbidden) {
			t.Fatal("unit may release protection or advertise unsupported boot behavior", forbidden)
		}
	}
}

func TestServicePlanRejectsHiddenPathsAndDirectiveInjection(t *testing.T) {
	value := systemd.Options{Executable: "/opt/nhop/client", Profile: "/etc/nhop/profile.json", Runtimes: "/etc/nhop/runtimes.json", Network: network.Defaults(0)}
	for _, path := range []string{"relative", "/etc/../profile", "/home/a/profile", "/tmp/profile", "/run/nhop-client/profile", "/etc/a\nExecStart=/bin/false", "/etc/a\x00"} {
		value.Profile = path
		if _, err := systemd.Unit(value); err == nil {
			t.Fatal("unsafe service input path was accepted", path)
		}
	}
	value.Profile = "/etc/nhop/profile.json"
	value.Network.IPv6 = "invalid"
	if _, err := systemd.Unit(value); err == nil {
		t.Fatal("invalid network policy accepted")
	}
}
