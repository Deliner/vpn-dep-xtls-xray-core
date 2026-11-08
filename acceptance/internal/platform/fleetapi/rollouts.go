package fleetapi

import (
	"context"
	"fmt"
	"net/http"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/validate"
)

func (c *Client) StageCandidate(ctx context.Context, nodeID, token string, manifest []byte) (wire.Candidate, error) {
	if validate.Name(nodeID) != nil {
		return wire.Candidate{}, fmt.Errorf("invalid candidate node ID")
	}
	raw, err := c.request(ctx, http.MethodPost, "/v1/admin/candidates/"+nodeID, token, wire.CandidateUpload{Manifest: manifest}, 16<<10)
	var value wire.Candidate
	if err == nil {
		err = decode(raw, &value)
	}
	return value, err
}

func (c *Client) CreateRollout(ctx context.Context, token string, spec wire.RolloutSpec) (wire.Rollout, error) {
	raw, err := c.request(ctx, http.MethodPost, "/v1/admin/rollouts", token, spec, 16<<20)
	var value wire.Rollout
	if err == nil {
		err = decode(raw, &value)
	}
	return value, err
}

func (c *Client) Rollouts(ctx context.Context, token string) ([]wire.Rollout, error) {
	raw, err := c.request(ctx, http.MethodGet, "/v1/admin/rollouts", token, nil, 16<<20)
	var value []wire.Rollout
	if err == nil {
		err = decode(raw, &value)
	}
	return value, err
}

func (c *Client) Rollout(ctx context.Context, id, token string) (wire.Rollout, error) {
	return c.rollout(ctx, http.MethodGet, id, "", token)
}

func (c *Client) CancelRollout(ctx context.Context, id, token string) (wire.Rollout, error) {
	return c.rollout(ctx, http.MethodPost, id, "/cancel", token)
}

func (c *Client) rollout(ctx context.Context, method, id, suffix, token string) (wire.Rollout, error) {
	if validate.Name(id) != nil {
		return wire.Rollout{}, fmt.Errorf("invalid rollout ID")
	}
	raw, err := c.request(ctx, method, "/v1/admin/rollouts/"+id+suffix, token, nil, 16<<20)
	var value wire.Rollout
	if err == nil {
		err = decode(raw, &value)
	}
	return value, err
}
