package bootstrap

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"vpn-script/internal/control/client/guard"
	"vpn-script/internal/platform/safeio"
)

type kernel struct {
	t         *testing.T
	state     string
	current   guard.Lease
	failure   string
	failed    bool
	creations int
}

func fixture(t *testing.T, failure string) (*store, *kernel) {
	t.Helper()
	base := filepath.Join(t.TempDir(), "bootstrap")
	k := &kernel{t: t, state: filepath.Join(base, "state.json"), failure: failure}
	s, err := openStore(base, true, k)
	require(t, err)
	t.Cleanup(s.close)
	return s, k
}

func (k *kernel) Verify(_ context.Context, lease guard.Lease) error {
	if lease != k.current {
		return errors.New("kernel table does not match the lease")
	}
	return nil
}

func (k *kernel) Resolve(ctx context.Context, lease guard.Lease) (guard.Lease, error) {
	if lease.Handle != 0 || lease.Token == "" {
		return lease, k.Verify(ctx, lease)
	}
	if k.current == (guard.Lease{}) {
		return guard.Lease{}, nil
	}
	if lease.Token != k.current.Token {
		return guard.Lease{}, errors.New("another table owns this name")
	}
	return k.current, nil
}

func (k *kernel) AcquireBootstrap(_ context.Context, lease guard.Lease) (guard.Lease, error) {
	var saved journal
	require(k.t, safeio.LoadJSON(k.state, &saved))
	if saved.Guard != lease || lease.Handle != 0 || saved.Releasing {
		k.t.Fatal("kernel mutation preceded its durable acquisition intent")
	}
	if err := k.crash("before-create"); err != nil {
		return guard.Lease{}, err
	}
	if k.current != (guard.Lease{}) {
		return guard.Lease{}, errors.New("table exists")
	}
	k.creations++
	k.current = guard.Lease{Token: lease.Token, Handle: uint64(k.creations)}
	return k.current, k.crash("after-create")
}

func (k *kernel) Release(ctx context.Context, lease guard.Lease) error {
	var saved journal
	require(k.t, safeio.LoadJSON(k.state, &saved))
	if !saved.Releasing || saved.Guard != lease {
		k.t.Fatal("kernel deletion preceded its durable release intent")
	}
	if err := k.crash("before-delete"); err != nil {
		return err
	}
	if err := k.Verify(ctx, lease); err != nil {
		return err
	}
	k.current = guard.Lease{}
	return k.crash("after-delete")
}

func (k *kernel) crash(boundary string) error {
	if k.failure == boundary && !k.failed {
		k.failed = true
		return errors.New("injected process interruption at " + boundary)
	}
	return nil
}

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
