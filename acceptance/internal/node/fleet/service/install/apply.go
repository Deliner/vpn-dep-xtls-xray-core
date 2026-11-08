package install

import (
	"context"

	"vpn-script/internal/node/fleet/pull"
	"vpn-script/internal/node/fleet/service"
)

// Apply requires an idle, disabled unit and an initialized agent journal. It
// publishes a private binary snapshot and unit; Start separately enables it.
func Apply(ctx context.Context, options Options) (Receipt, error) {
	if err := linuxRoot(); err != nil {
		return Receipt{}, err
	}
	if err := validateStatePath(options.State); err != nil {
		return Receipt{}, err
	}
	if _, err := service.Unit(service.Directory+"/nhop-fleet", options.State, options.Interval); err != nil {
		return Receipt{}, err
	}
	owner, err := pull.Open(options.State)
	if err != nil {
		return Receipt{}, err
	}
	defer owner.Close()
	binary, err := runningBinary()
	if err != nil {
		return Receipt{}, err
	}
	t, err := openTransaction(true)
	if err != nil {
		return Receipt{}, err
	}
	defer t.store.lock.Close()
	return t.apply(ctx, options, binary)
}

func openTransaction(create bool) (transaction, error) {
	manager, err := systemManager()
	if err != nil {
		return transaction{}, err
	}
	s, err := openStore(service.Directory, service.UnitPath, create)
	return transaction{store: s, manager: manager}, err
}

func (t transaction) apply(ctx context.Context, options Options, binary []byte) (Receipt, error) {
	_, previous, err := t.store.current()
	if err != nil {
		return Receipt{}, err
	}
	if err := t.inactive(ctx, "either"); err != nil {
		return Receipt{}, err
	}
	id, err := t.store.publish(options, binary)
	if err != nil {
		return Receipt{}, err
	}
	_, desired, err := t.store.revision(id)
	if err != nil {
		return Receipt{}, err
	}
	return Receipt{Generation: id, Unit: service.Name, State: options.State}, t.publish(ctx, previous, desired)
}
