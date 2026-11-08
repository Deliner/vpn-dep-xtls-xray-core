package boot

import (
	"context"
	"fmt"
)

func (t transaction) selectManager(ctx context.Context, requested string) (string, error) {
	if requested != "" && !validManager(requested) {
		return "", fmt.Errorf("supported network managers: systemd-networkd.service or NetworkManager.service")
	}
	selected := ""
	for _, unit := range []string{"systemd-networkd.service", "NetworkManager.service"} {
		value, err := t.show(ctx, unit, properties)
		if err != nil {
			return "", err
		}
		if value["ActiveState"] != "active" {
			continue
		}
		if value["LoadState"] != "loaded" || value["SubState"] != "running" || value["Job"] != "" || selected != "" {
			return "", fmt.Errorf("network manager is ambiguous, transitioning or unsupported")
		}
		selected = unit
	}
	if selected == "" || requested != "" && selected != requested {
		return "", fmt.Errorf("one supported active network manager matching the requested selection is required")
	}
	return selected, nil
}
