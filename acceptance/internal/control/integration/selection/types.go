package selection

import "vpn-script/internal/control/integration/evidence"

// Options separates the default servers, external clients and selected updated nodes.
type Options struct {
	Servers      map[string]evidence.Input
	Clients      map[string]evidence.Input
	Updated      map[string]evidence.Input
	UpdatedNodes []string
}

// Set binds every participant to an identity that will be captured before launch.
type Set struct {
	Inputs  map[string]evidence.Input    `json:"-"`
	Clients map[string]string            `json:"clients"`
	Nodes   map[string]map[string]string `json:"nodes"`
}
