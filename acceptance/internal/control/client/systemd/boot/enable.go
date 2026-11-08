package boot

import (
	"context"
	"fmt"
)

func (t transaction) enable(ctx context.Context, requested string) (Receipt, error) {
	value, found, err := t.read()
	if err != nil {
		return Receipt{}, err
	}
	if !found {
		value, err = t.begin(ctx, requested)
		if err != nil {
			return Receipt{}, err
		}
	}
	if value.Phase == "disabling" || requested != "" && requested != value.NetworkManager {
		return Receipt{}, fmt.Errorf("retained boot intent differs; complete service-disable before changing it")
	}
	files, err := t.artifacts(value)
	if err != nil {
		return Receipt{}, err
	}
	if err := t.inspect(files, value.Phase); err != nil {
		return Receipt{}, err
	}
	if err := t.publish(ctx, files, value); err != nil {
		return Receipt{}, fmt.Errorf("boot intent retained; retry service-enable or service-disable: %w", err)
	}
	value.Phase = "enabled"
	return t.receipt(value, true), t.write(value, false)
}

func (t transaction) begin(ctx context.Context, requested string) (record, error) {
	if err := t.client(ctx, false); err != nil {
		return record{}, err
	}
	if err := t.absentGuard(ctx); err != nil {
		return record{}, err
	}
	selected, err := t.selectManager(ctx, requested)
	if err != nil {
		return record{}, err
	}
	value := record{SchemaVersion: 1, OwnerToken: t.snapshot.OwnerToken,
		Generation: t.snapshot.Generation, NetworkManager: selected, Phase: "enabling"}
	if err := t.noArtifacts(); err != nil {
		return record{}, err
	}
	return value, t.write(value, true)
}

func (t transaction) publish(ctx context.Context, files []artifact, value record) error {
	for _, item := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := t.create(item); err != nil {
			return err
		}
	}
	if _, err := t.command(ctx, "daemon-reload"); err != nil {
		return err
	}
	return t.enabled(ctx, value)
}
