package sockets

import "net/netip"

// Record describes one observed kernel listener, not a desired engine configuration.
type Record struct {
	Address   netip.Addr
	Interface string
	Network   string
	Port      int
	DualStack bool
	PIDs      []string
}
