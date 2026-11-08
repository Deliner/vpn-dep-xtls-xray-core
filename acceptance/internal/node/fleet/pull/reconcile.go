package pull

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/platform/safeio"
)

func (r *Runner) Step(ctx context.Context) (wire.Status, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lock == nil {
		return wire.Status{}, ErrClosed
	}
	if err := r.enroll(ctx); err != nil {
		return wire.Status{}, err
	}
	raw, err := r.client.Assignment(ctx, r.config.Trust.NodeID, r.config.Credential)
	if err != nil {
		return wire.Status{}, err
	}
	var status wire.Status
	if len(raw) == 0 {
		status, err = r.agent.Status()
	} else {
		status, err = r.agent.Apply(ctx, raw)
	}
	if status.NodeID == "" {
		return status, err
	}
	reportErr := r.client.Report(ctx, r.config.Credential, status)
	return status, errors.Join(err, reportErr)
}

func (r *Runner) enroll(ctx context.Context) error {
	if r.config.Enrolled {
		return nil
	}
	request := wire.Enrollment{NodeID: r.config.Trust.NodeID, Token: r.config.EnrollmentToken, Credential: r.config.Credential}
	identity, err := r.client.Enroll(ctx, request)
	if err != nil {
		return err
	}
	if identity.FleetID != r.config.Trust.FleetID || identity.NodeID != r.config.Trust.NodeID {
		return fmt.Errorf("Fleet enrollment response belongs to another identity")
	}
	next := r.config
	next.Enrolled, next.EnrollmentToken = true, ""
	if err := safeio.JSON(filepath.Join(r.directory, "connection.json"), next); err != nil {
		return err
	}
	r.config = next
	return nil
}
