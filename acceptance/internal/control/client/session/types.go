// Package session owns an explicitly started Linux client and its fail-closed
// shutdown. Callers serialize methods and protect the parent of Directory.
package session

import (
	"os"

	"vpn-script/internal/control/client/guard"
	"vpn-script/internal/control/client/network"
	"vpn-script/internal/control/client/profile"
	"vpn-script/internal/control/client/resolver"
	"vpn-script/internal/control/client/routing"
)

type Artifact struct {
	Path   string
	SHA256 string
}

type Request struct {
	Directory string
	Profile   profile.Imported
	Network   network.Options
	EngineGID int
	Native    Artifact
	Frontend  Artifact
	Cronet    Artifact
}

// Status deliberately excludes credentials, configuration and engine logs.
type Status struct {
	Phase       string        `json:"phase"`
	NativePID   int           `json:"native_pid"`
	FrontendPID int           `json:"frontend_pid"`
	Link        resolver.Link `json:"link"`
	Guard       guard.Lease   `json:"guard"`
}

type Session struct {
	request   Request
	status    Status
	guard     *guard.Manager
	dns       *resolver.Manager
	group     *children
	lock      *os.File
	protected bool
	routes    *routing.Manager
}

func (s *Session) Status() Status { return s.status }
