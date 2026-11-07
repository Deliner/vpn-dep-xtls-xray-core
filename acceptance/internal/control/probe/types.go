package probe

import (
	"context"
	"os/exec"

	"vpn-script/internal/core/model"
)

type probeClient struct {
	port    int
	ctx     context.Context
	cancel  context.CancelCauseFunc
	command *exec.Cmd
	done    chan struct{}
}

type SSHResult struct {
	Port          int              `json:"port"`
	Authenticated bool             `json:"authenticated"`
	Closed        map[string]*bool `json:"closed_ports_reachable_from_controller"`
	Pass          bool             `json:"pass"`
}

type RouteResult struct {
	Actual   string `json:"actual_exit_ip,omitempty"`
	Expected string `json:"expected_exit_ip,omitempty"`
	Pass     bool   `json:"pass"`
	Scope    string `json:"scope"`
	Error    string `json:"error,omitempty"`
}

type Report struct {
	VerifiedAt string                 `json:"verified_at"`
	SSH        map[string]SSHResult   `json:"ssh"`
	Routes     map[string]RouteResult `json:"routes"`
}

type ClientCase struct {
	Profile model.Profile
	Config  model.Object
}
