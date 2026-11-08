package fleetapi

import (
	"context"
	"fmt"
	"net/http"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/validate"
)

func (c *Client) Invite(ctx context.Context, token string, request wire.Invite) (wire.Invitation, error) {
	raw, err := c.request(ctx, http.MethodPost, "/v1/admin/invitations", token, request, 16<<10)
	var value wire.Invitation
	if err == nil {
		err = decode(raw, &value)
	}
	return value, err
}

func (c *Client) List(ctx context.Context, token string) ([]wire.Node, error) {
	raw, err := c.request(ctx, http.MethodGet, "/v1/admin/nodes", token, nil, 16<<20)
	var value []wire.Node
	if err == nil {
		err = decode(raw, &value)
	}
	return value, err
}

func (c *Client) Node(ctx context.Context, nodeID, token string) (wire.Node, error) {
	if validate.Name(nodeID) != nil {
		return wire.Node{}, fmt.Errorf("invalid Fleet node ID")
	}
	raw, err := c.request(ctx, http.MethodGet, "/v1/admin/nodes/"+nodeID, token, nil, 16<<10)
	var value wire.Node
	if err == nil {
		err = decode(raw, &value)
	}
	return value, err
}

func (c *Client) Publish(ctx context.Context, nodeID, token string, request wire.Publish) (wire.Desired, error) {
	if validate.Name(nodeID) != nil {
		return wire.Desired{}, fmt.Errorf("invalid Fleet node ID")
	}
	raw, err := c.request(ctx, http.MethodPost, "/v1/admin/nodes/"+nodeID+"/assignment", token, request, 16<<10)
	var value wire.Desired
	if err == nil {
		err = decode(raw, &value)
	}
	return value, err
}

func (c *Client) Revoke(ctx context.Context, nodeID, token string) error {
	if validate.Name(nodeID) != nil {
		return fmt.Errorf("invalid Fleet node ID")
	}
	_, err := c.request(ctx, http.MethodPost, "/v1/admin/nodes/"+nodeID+"/revoke", token, nil, 16<<10)
	return err
}
