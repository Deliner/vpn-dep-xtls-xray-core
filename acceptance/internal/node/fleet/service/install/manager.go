package install

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"vpn-script/internal/node/fleet/service"
)

const properties = "LoadState,ActiveState,SubState,FragmentPath,DropInPaths,UnitFileState,Job"

func systemManager() (manager, error) {
	path, err := protectedExecutable("/usr/bin/systemctl")
	if err != nil {
		return nil, err
	}
	return func(parent context.Context, args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(parent, 150*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, path, args...)
		command.Env = []string{"PATH=/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL=C", "SYSTEMD_PAGER=cat", "SYSTEMD_COLORS=0"}
		return command.Output()
	}, nil
}

func (t transaction) inspect(ctx context.Context) (map[string]string, error) {
	raw, commandErr := t.manager(ctx, "show", "--property="+properties, "--", service.Name)
	value, err := parseState(raw)
	if err != nil || value["DropInPaths"] != "" {
		return nil, fmt.Errorf("cannot inspect Fleet unit or unexpected overrides exist")
	}
	if value["LoadState"] == "not-found" && value["FragmentPath"] == "" && value["UnitFileState"] == "" {
		return value, nil
	}
	if commandErr != nil || value["LoadState"] != "loaded" || value["FragmentPath"] != t.store.unit {
		return nil, fmt.Errorf("refusing foreign or invalid Fleet systemd unit")
	}
	if value["UnitFileState"] != "disabled" && value["UnitFileState"] != "enabled" {
		return nil, fmt.Errorf("Fleet unit is masked, linked, transient or unexpectedly enabled")
	}
	return value, nil
}

func (t transaction) inactive(ctx context.Context, expected string) error {
	value, err := t.inspect(ctx)
	if err != nil {
		return err
	}
	if value["ActiveState"] != "inactive" || value["SubState"] != "dead" || value["Job"] != "" || value["UnitFileState"] == "enabled" {
		return fmt.Errorf("Fleet unit must be stopped and disabled before installation or removal")
	}
	if expected == "present" && value["LoadState"] != "loaded" || expected == "absent" && value["LoadState"] != "not-found" {
		return fmt.Errorf("Fleet unit presence differs after daemon-reload")
	}
	return nil
}

func parseState(raw []byte) (map[string]string, error) {
	if len(raw) > 65536 {
		return nil, fmt.Errorf("systemd response exceeds size limit")
	}
	value := map[string]string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		key, item, ok := strings.Cut(line, "=")
		if _, duplicate := value[key]; !ok || duplicate {
			return nil, fmt.Errorf("invalid or duplicate systemd property")
		}
		value[key] = item
	}
	for _, key := range strings.Split(properties, ",") {
		if _, ok := value[key]; !ok {
			return nil, fmt.Errorf("missing systemd property")
		}
	}
	if len(value) != 7 {
		return nil, fmt.Errorf("unexpected systemd property")
	}
	return value, nil
}
