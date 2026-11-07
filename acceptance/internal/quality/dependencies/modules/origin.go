package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"vpn-script/internal/quality/dependencies/commands"
)

func originRequests(modules []Module) ([]string, map[string][]*Module) {
	targets := map[string][]*Module{}
	var requests []string
	for index := range modules {
		module := &modules[index]
		if module.Main {
			continue
		}
		if module.Replace != nil {
			module = module.Replace
		}
		key := module.Path + "@" + module.Version
		if targets[key] == nil {
			requests = append(requests, key)
		}
		targets[key] = append(targets[key], module)
	}
	sort.Strings(requests)
	return requests, targets
}

func resolveOrigins(ctx context.Context, run commands.Runner, directory string, modules []Module) error {
	requests, targets := originRequests(modules)
	if len(requests) == 0 {
		return nil
	}
	args := []string{"-C", directory, "list", "-mod=readonly", "-m", "-json"}
	raw, err := run(ctx, "go", append(args, requests...)...)
	if err != nil {
		return err
	}
	return applyOrigins(raw, targets)
}

func applyOrigins(raw []byte, targets map[string][]*Module) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	for {
		var source Module
		if err := decoder.Decode(&source); err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		key := source.Path + "@" + source.Version
		modules, found := targets[key]
		if !found || source.Error != nil {
			return fmt.Errorf("unexpected origin query result %q", key)
		}
		for _, module := range modules {
			module.Origin = source.Origin
		}
		delete(targets, key)
	}
	if len(targets) != 0 {
		return fmt.Errorf("origin query omitted %d requested modules", len(targets))
	}
	return nil
}
