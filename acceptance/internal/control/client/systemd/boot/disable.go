package boot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"vpn-script/internal/control/client/systemd"
	"vpn-script/internal/platform/safeio"
)

func (t transaction) disable(ctx context.Context) (Receipt, error) {
	value, found, err := t.read()
	if err != nil {
		return Receipt{}, err
	}
	if !found {
		return Receipt{Generation: t.snapshot.Generation}, t.unregistered(ctx)
	}
	files, err := t.artifacts(value)
	if err != nil {
		return Receipt{}, err
	}
	if err := t.inspect(files, value.Phase); err != nil {
		return Receipt{}, err
	}
	value.Phase = "disabling"
	if err := t.write(value, false); err != nil {
		return Receipt{}, err
	}
	if err := t.unpublish(ctx, files); err != nil {
		return Receipt{}, fmt.Errorf("boot removal intent retained; retry service-disable: %w", err)
	}
	if err := os.Remove(t.state); err != nil {
		return Receipt{}, err
	}
	return t.receipt(value, false), safeio.SyncDir(filepath.Dir(t.state))
}

func (t transaction) unpublish(ctx context.Context, files []artifact) error {
	for index := len(files) - 1; index > 0; index-- {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := t.remove(files[index]); err != nil {
			return err
		}
	}
	if _, err := t.command(ctx, "daemon-reload"); err != nil {
		return err
	}
	if err := t.stopGuard(ctx); err != nil {
		return err
	}
	if err := t.remove(files[0]); err != nil {
		return err
	}
	if _, err := t.command(ctx, "daemon-reload"); err != nil {
		return err
	}
	return t.unregistered(ctx)
}

func (t transaction) stopGuard(ctx context.Context) error {
	fields := "LoadState,ActiveState,SubState,FragmentPath,DropInPaths,Job,RequiredBy,BoundBy,ConsistsOf,PropagatesStopTo,RequisiteOf"
	value, err := t.show(ctx, systemd.BootService, fields)
	if err != nil {
		return err
	}
	for _, key := range []string{"DropInPaths", "Job", "RequiredBy", "BoundBy", "ConsistsOf", "PropagatesStopTo", "RequisiteOf"} {
		if value[key] != "" {
			return fmt.Errorf("early protection unit still has dependencies, a job or overrides; refusing stop")
		}
	}
	if value["LoadState"] == "not-found" && value["ActiveState"] == "inactive" && value["FragmentPath"] == "" {
		return nil
	}
	if value["LoadState"] != "loaded" || value["FragmentPath"] != filepath.Join(t.units, systemd.BootService) {
		return fmt.Errorf("cannot identify retained early protection service")
	}
	if value["ActiveState"] == "inactive" && value["SubState"] == "dead" {
		return nil
	}
	if value["ActiveState"] != "failed" && (value["ActiveState"] != "active" || value["SubState"] != "exited") {
		return fmt.Errorf("early protection service is transitioning; inspect before disabling")
	}
	operation := "stop"
	if value["ActiveState"] == "failed" {
		operation = "reset-failed"
	}
	_, err = t.command(ctx, operation, "--", systemd.BootService)
	return err
}
