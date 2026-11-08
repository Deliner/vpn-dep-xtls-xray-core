package fleet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"time"

	contract "vpn-script/internal/core/fleet"
	"vpn-script/internal/core/model"
	"vpn-script/internal/node/installer"
	"vpn-script/internal/node/manifest"
	"vpn-script/internal/platform/safeio"
)

func New(directory string, trust contract.Trust) *Agent {
	trust.PublicKey = bytes.Clone(trust.PublicKey)
	return &Agent{directory: directory, trust: trust, now: time.Now,
		install: func(ctx context.Context, data model.NodeConfig) (installer.Result, error) {
			return installer.New(ctx).Install(data)
		}}
}

func (a *Agent) Apply(ctx context.Context, raw []byte) (Status, error) {
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}
	if err := a.checkDirectory(); err != nil {
		return Status{}, err
	}
	lock, err := safeio.Lock(filepath.Join(a.directory, "agent.lock"))
	if err != nil {
		return Status{}, err
	}
	defer lock.Close()
	previous, err := a.loadState()
	if err != nil {
		return Status{}, err
	}
	value, receipt, err := contract.Accept(raw, a.trust, previous.Accepted, a.now())
	if err != nil {
		return previous.status(), err
	}
	data, err := manifest.Decode(value.Manifest)
	if err != nil {
		return previous.status(), err
	}
	if data.NodeID != value.NodeID {
		return previous.status(), fmt.Errorf("signed manifest belongs to another node")
	}
	return a.applyAccepted(ctx, previous, receipt, data)
}

func (a *Agent) applyAccepted(ctx context.Context, next state, receipt contract.Receipt, data model.NodeConfig) (Status, error) {
	if err := ctx.Err(); err != nil {
		return next.status(), err
	}
	if next.Attempt == math.MaxUint64 {
		return next.status(), fmt.Errorf("Fleet attempt counter exhausted")
	}
	next.Attempt++
	next.Accepted, next.Phase, next.AttemptedAt = receipt, "applying", a.now().Unix()
	if err := a.saveState(next); err != nil {
		return Status{}, err
	}
	result, err := a.install(ctx, data)
	if err == nil && (result.Node != data.NodeID || result.Generation != data.Generation) {
		err = fmt.Errorf("installer result identity mismatch")
	}
	if err != nil {
		next.Phase = "failed"
		return next.status(), errors.Join(err, a.saveState(next))
	}
	next.Applied, next.Generation, next.Phase = receipt, data.Generation, "ready"
	return next.status(), a.saveState(next)
}
