package peer

import (
	"sync"

	"vpn-script/internal/quality/linuxlab/reboot/contract"
)

// All listeners share the same counters; only handlers mutate them under mu.
type server struct {
	config  contract.Config
	mu      sync.Mutex
	metrics contract.PeerMetrics
}
