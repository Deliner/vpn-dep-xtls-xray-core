// Package fleetapi is the bounded HTTPS transport for the Fleet wire contract.
// It does not sign assignments, persist credentials or apply node manifests.
package fleetapi

import (
	"fmt"
	"net/http"
)

type Client struct {
	base string
	http *http.Client
}

// HTTPError deliberately excludes response bodies, which are untrusted and may
// contain credentials or local server diagnostics.
type HTTPError struct {
	Status int
}

func (e *HTTPError) Error() string { return fmt.Sprintf("Fleet API returned HTTP %d", e.Status) }

func (c *Client) Close() { c.http.CloseIdleConnections() }
