// Package guard owns one persistent, explicitly released nftables kill switch.
package guard

import (
	"fmt"
	"os"
	"runtime"
	"vpn-script/internal/platform/process"
)

type Lease struct {
	Token  string `json:"token"`
	Handle uint64 `json:"handle"`
}

type Manager struct{ run process.Runner }

func New() (*Manager, error) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return nil, fmt.Errorf("kill switch activation requires Linux root")
	}
	return &Manager{run: process.Run}, nil
}
