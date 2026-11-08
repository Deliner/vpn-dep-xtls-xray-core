// Package controller serializes a Linux session and exposes a root-only local
// command socket. Neither an engine exit nor controller cancellation opens egress.
package controller

import (
	"vpn-script/internal/control/client/guard/bootstrap"
	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/session"
)

type Options struct {
	Profile  string
	Runtimes string
	Network  network.Options
}

type Runtimes struct {
	SchemaVersion int              `json:"schema_version"`
	Xray          session.Artifact `json:"xray"`
	SingBox       session.Artifact `json:"sing_box"`
	Cronet        session.Artifact `json:"cronet"`
}

// Diagnostic intentionally excludes secrets, runtime configuration and raw logs.
// Running means local readiness; it is not proof of remote exit reachability.
type Diagnostic struct {
	Controller string            `json:"controller"`
	Directory  string            `json:"directory,omitempty"`
	Status     session.Status    `json:"status"`
	Error      string            `json:"error,omitempty"`
	Bootstrap  *bootstrap.Status `json:"bootstrap,omitempty"`
}

type request struct {
	Operation string `json:"operation"`
}

type call struct {
	operation string
	reply     chan Diagnostic
	finished  chan struct{}
}
