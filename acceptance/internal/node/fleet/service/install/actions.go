package install

import (
	"context"
	"fmt"

	"vpn-script/internal/node/fleet/service"
)

func Control(ctx context.Context, action string) error {
	if err := linuxRoot(); err != nil {
		return err
	}
	if action != "start" && action != "stop" && action != "remove" {
		return fmt.Errorf("unknown Fleet service operation")
	}
	t, err := openTransaction(false)
	if err != nil {
		return err
	}
	defer t.store.lock.Close()
	_, previous, err := t.store.current()
	if err != nil || previous == nil {
		return fmt.Errorf("owned Fleet service is not installed or cannot be verified")
	}
	if action == "remove" {
		if err := t.inactive(ctx, "either"); err != nil {
			return err
		}
		return t.publish(ctx, previous, nil)
	}
	return t.activate(ctx, action)
}

func (t transaction) activate(ctx context.Context, action string) error {
	if _, err := t.inspect(ctx); err != nil {
		return err
	}
	verb := "enable"
	if action == "stop" {
		verb = "disable"
	}
	if _, err := t.manager(ctx, verb, "--now", "--", service.Name); err != nil {
		return fmt.Errorf("Fleet service %s failed: %w", action, err)
	}
	if action == "stop" {
		return t.stopped(ctx)
	}
	value, err := t.inspect(ctx)
	if err != nil {
		return err
	}
	if value["ActiveState"] != "active" || value["SubState"] != "running" || value["UnitFileState"] != "enabled" || value["Job"] != "" {
		return fmt.Errorf("Fleet agent did not reach enabled/running state; inspect its journal")
	}
	return nil
}

func (t transaction) stopped(ctx context.Context) error {
	value, err := t.inspect(ctx)
	if err != nil {
		return err
	}
	if value["ActiveState"] == "failed" && value["SubState"] == "failed" && value["UnitFileState"] == "disabled" && value["Job"] == "" {
		if _, err := t.manager(ctx, "reset-failed", "--", service.Name); err != nil {
			return fmt.Errorf("clear stopped Fleet service failure: %w", err)
		}
	}
	return t.inactive(ctx, "present")
}
