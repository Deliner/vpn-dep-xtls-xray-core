package install

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"vpn-script/internal/control/client/host"
)

const properties = "LoadState,ActiveState,SubState,FragmentPath,DropInPaths,UnitFileState,Job"

func systemManager() (manager, error) {
	path, err := host.ProtectedExecutable("/usr/bin/systemctl")
	if err != nil {
		return nil, err
	}
	return func(parent context.Context, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(parent, 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, path, args...)
		// Ignore remote/user bus, pager, unit-path and loader overrides. All calls
		// address the local system manager and fixed unit name only.
		command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL=C", "SYSTEMD_PAGER=cat", "SYSTEMD_COLORS=0"}
		return command.Output()
	}, nil
}

func (t transaction) inactive(ctx context.Context, expected string) error {
	data, commandErr := t.manager(ctx, "show", "--property="+properties, "--", Service)
	state, err := parseState(data)
	if err != nil {
		return fmt.Errorf("cannot inspect client systemd state: %w", err)
	}
	if state["ActiveState"] != "inactive" || state["SubState"] != "dead" || state["DropInPaths"] != "" || state["Job"] != "" {
		return fmt.Errorf("client unit is active, queued or overridden; explicitly disconnect and stop it before installation")
	}
	if state["LoadState"] == "not-found" && expected != "present" && state["FragmentPath"] == "" && state["UnitFileState"] == "" {
		return nil
	}
	if commandErr != nil || expected == "absent" || state["LoadState"] != "loaded" || state["FragmentPath"] != t.store.unit || state["UnitFileState"] != "static" {
		return fmt.Errorf("refusing unknown, enabled, failed or foreign systemd client unit")
	}
	return nil
}

func parseState(data []byte) (map[string]string, error) {
	if len(data) > 65536 {
		return nil, fmt.Errorf("systemd state exceeds size limit")
	}
	value := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		key, item, found := strings.Cut(line, "=")
		if _, duplicate := value[key]; !found || duplicate {
			return nil, fmt.Errorf("invalid or duplicate systemd property")
		}
		value[key] = item
	}
	for _, key := range strings.Split(properties, ",") {
		if _, found := value[key]; !found {
			return nil, fmt.Errorf("missing systemd property: %s", key)
		}
	}
	if len(value) != 7 {
		return nil, fmt.Errorf("unexpected systemd property")
	}
	return value, nil
}
