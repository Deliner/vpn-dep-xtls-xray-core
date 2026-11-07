// Package resolver owns per-link systemd-resolved policy for a newly created TUN.
// The caller owns that link and must retain it until Revert has completed.
package resolver

import (
	"fmt"
	"net"
	"runtime"

	"vpn-script/internal/platform/process"
)

type Link struct {
	Name  string `json:"name"`
	Index int    `json:"index"`
}

type Manager struct {
	run    process.Runner
	lookup func(string) (*net.Interface, error)
}

// New refuses platforms where the project's client must not change networking.
func New() (*Manager, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("system DNS management is supported only on Linux")
	}
	return &Manager{run: process.Run, lookup: net.InterfaceByName}, nil
}
