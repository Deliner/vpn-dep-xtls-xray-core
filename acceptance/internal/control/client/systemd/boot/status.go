package boot

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"vpn-script/internal/control/client/systemd"
	"vpn-script/internal/control/client/systemd/install"
	"vpn-script/internal/control/client/systemd/manager"
)

func (t transaction) show(ctx context.Context, unit, fields string) (map[string]string, error) {
	data, commandErr := t.command(ctx, "show", "--property="+fields, "--", unit)
	value, err := manager.Parse(data, fields)
	if err != nil {
		return nil, err
	}
	if commandErr != nil && value["LoadState"] != "not-found" {
		return nil, fmt.Errorf("cannot inspect %s: %w", unit, commandErr)
	}
	return value, nil
}

func (t transaction) client(ctx context.Context, enabled bool) error {
	value, err := t.show(ctx, install.Service, properties)
	if err != nil {
		return err
	}
	drop, state := "", "static"
	if enabled {
		drop, state = filepath.Join(t.units, install.Service+".d", dropIn), "enabled"
	}
	if value["LoadState"] != "loaded" || value["FragmentPath"] != t.snapshot.Unit ||
		value["ActiveState"] != "inactive" || value["SubState"] != "dead" || value["Job"] != "" ||
		value["DropInPaths"] != drop || value["UnitFileState"] != state {
		return fmt.Errorf("client service is active, queued, overridden or has unexpected boot registration")
	}
	if enabled && !requiresGuard(value) {
		return fmt.Errorf("client service lacks required early protection ordering")
	}
	return nil
}

func requiresGuard(value map[string]string) bool {
	return slices.Contains(strings.Fields(value["Requires"]), systemd.BootService) &&
		slices.Contains(strings.Fields(value["After"]), systemd.BootService)
}

func (t transaction) enabled(ctx context.Context, value record) error {
	if err := t.client(ctx, true); err != nil {
		return err
	}
	guard, err := t.show(ctx, systemd.BootService, properties)
	if err != nil {
		return err
	}
	if guard["LoadState"] != "loaded" || guard["FragmentPath"] != filepath.Join(t.units, systemd.BootService) ||
		guard["DropInPaths"] != "" || guard["Job"] != "" || !settledGuard(guard) {
		return fmt.Errorf("early protection unit is missing, failed, queued or overridden")
	}
	return t.protectedManagers(ctx, value.NetworkManager)
}

func settledGuard(value map[string]string) bool {
	return value["ActiveState"] == "inactive" && value["SubState"] == "dead" ||
		value["ActiveState"] == "active" && value["SubState"] == "exited"
}

// Both supported managers are gated, including an inactive or not-yet-installed
// alternative. Switching managers must not bypass the boot protection service.
func (t transaction) protectedManagers(ctx context.Context, selected string) error {
	for _, unit := range []string{"systemd-networkd.service", "NetworkManager.service"} {
		value, err := t.show(ctx, unit, properties)
		if err != nil {
			return err
		}
		if unit != selected && value["LoadState"] == "not-found" {
			continue
		}
		if value["LoadState"] != "loaded" || value["Job"] != "" || !requiresGuard(value) {
			return fmt.Errorf("network manager %s lacks required early protection ordering or is transitioning", unit)
		}
	}
	return nil
}

func (t transaction) absentGuard(ctx context.Context) error {
	value, err := t.show(ctx, systemd.BootService, properties)
	if err != nil {
		return err
	}
	if value["LoadState"] != "not-found" || value["ActiveState"] != "inactive" || value["SubState"] != "dead" ||
		value["FragmentPath"] != "" || value["DropInPaths"] != "" || value["Job"] != "" {
		return fmt.Errorf("unrecorded early protection unit exists; inspect before registration")
	}
	return nil
}

func (t transaction) unregistered(ctx context.Context) error {
	if err := t.noArtifacts(); err != nil {
		return err
	}
	if err := t.client(ctx, false); err != nil {
		return err
	}
	return t.absentGuard(ctx)
}
