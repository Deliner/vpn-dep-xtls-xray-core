// Package guard owns one persistent, explicitly released nftables kill switch.
package guard

import (
	"fmt"
	"os"
	"runtime"
	"vpn-script/internal/control/client/network"
	"vpn-script/internal/platform/process"
)

type Lease struct {
	Token  string `json:"token"`
	Handle uint64 `json:"handle"`
}

type Manager struct {
	run       process.Runner
	bootstrap bool
}

func New() (*Manager, error) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return nil, fmt.Errorf("kill switch activation requires Linux root")
	}
	return &Manager{run: process.Run}, nil
}

// NewBootstrap owns a separate table, with the same exclusive lease contract.
func NewBootstrap() (*Manager, error) {
	m, err := New()
	if err == nil {
		m.bootstrap = true
	}
	return m, err
}

func (m *Manager) tableName() string {
	if m.bootstrap {
		return network.BootTable
	}
	return network.Table
}
