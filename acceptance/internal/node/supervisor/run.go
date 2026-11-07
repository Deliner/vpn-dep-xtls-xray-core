package supervisor

import (
	"context"
	"fmt"
	"path/filepath"
	"vpn-script/internal/platform/safeio"
)

// Run pins the resolved release directory before launching children. A failed engine
// terminates the whole group; systemd restarts it without any alternate/direct route.
func Run(ctx context.Context, directory string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	directory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return err
	}
	var names []string
	if err := safeio.LoadJSON(filepath.Join(directory, "processes.json"), &names); err != nil {
		return err
	}
	if err := validateNames(names); err != nil {
		return err
	}
	group, err := start(ctx, directory, names)
	if err != nil {
		return err
	}
	defer group.stop()
	return waitForExit(ctx, group.exits)
}

func waitForExit(ctx context.Context, exits <-chan exitResult) error {
	var result exitResult
	select {
	case <-ctx.Done():
	case result = <-exits:
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if result.err != nil {
		return fmt.Errorf("engine %s stopped: %w", result.name, result.err)
	}
	return fmt.Errorf("engine %s stopped unexpectedly", result.name)
}

func validateNames(names []string) error {
	seen := map[string]bool{}
	if len(names) == 0 || len(names) > 2 {
		return fmt.Errorf("invalid process list")
	}
	for _, name := range names {
		if seen[name] || (name != "xray" && name != "sing-box") {
			return fmt.Errorf("invalid process name")
		}
		seen[name] = true
	}
	return nil
}
