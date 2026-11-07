package validate

import (
	"net/netip"

	"vpn-script/internal/core/model"
)

// SocketsOverlap compares concrete TCP/UDP bindings, independent of tags and
// visibility labels. Wildcards are conservative because dual-stack behavior
// depends on the engine and host; IPv4-mapped addresses share IPv4 sockets.
func SocketsOverlap(left, right model.Listener) bool {
	if left.Network != right.Network || left.Port != right.Port {
		return false
	}
	a, aErr := netip.ParseAddr(left.Address)
	b, bErr := netip.ParseAddr(right.Address)
	if aErr != nil || bErr != nil {
		return true
	}
	// Interface suffixes alone do not prove that an engine creates disjoint binds.
	a, b = a.Unmap().WithZone(""), b.Unmap().WithZone("")
	return a == b || a.IsUnspecified() || b.IsUnspecified()
}
