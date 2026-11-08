package fleetapi

import (
	"context"
	"fmt"
	"net/http"

	"vpn-script/internal/core/fleet/wire"
	"vpn-script/internal/core/validate"
)

func (c *Client) Enroll(ctx context.Context, request wire.Enrollment) (wire.Identity, error) {
	raw, err := c.request(ctx, http.MethodPost, "/v1/enroll", "", request, 16<<10)
	var value wire.Identity
	if err == nil {
		err = decode(raw, &value)
	}
	return value, err
}

func (c *Client) Assignment(ctx context.Context, nodeID, token string) ([]byte, error) {
	if validate.Name(nodeID) != nil {
		return nil, fmt.Errorf("invalid Fleet node ID")
	}
	return c.request(ctx, http.MethodGet, "/v1/nodes/"+nodeID+"/assignment", token, nil, 24<<20)
}

func (c *Client) Report(ctx context.Context, token string, value wire.Status) error {
	if validate.Name(value.NodeID) != nil {
		return fmt.Errorf("invalid Fleet node ID")
	}
	_, err := c.request(ctx, http.MethodPost, "/v1/nodes/"+value.NodeID+"/status", token, value, 16<<10)
	return err
}
