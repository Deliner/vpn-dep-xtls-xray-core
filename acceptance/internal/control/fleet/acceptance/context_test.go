package acceptance

import (
	"fmt"
	"testing"
)

func fleetMode(mode, systemd string) error {
	if mode != "private-lima" && mode != "owned-native-ci" {
		return fmt.Errorf("unsupported explicit Fleet acceptance context")
	}
	if mode != systemd {
		return fmt.Errorf("Fleet and node acceptance contexts must agree")
	}
	return nil
}

func TestFleetAcceptanceContextMustMatchTheOwnedNodeGate(t *testing.T) {
	for _, mode := range []string{"", "private-lima", "owned-native-ci", "foreign"} {
		for _, systemd := range []string{"", "private-lima", "owned-native-ci", "foreign"} {
			valid := mode == systemd && (mode == "private-lima" || mode == "owned-native-ci")
			if err := fleetMode(mode, systemd); (err == nil) != valid {
				t.Fatalf("context pair %q / %q: %v", mode, systemd, err)
			}
		}
	}
}
