package sockets

import (
	"net/netip"

	"vpn-script/internal/core/model"
)

// Conflicts is conservative for desired wildcards: Go engines may turn either
// wildcard family into a dual-stack listener. Concrete addresses stay distinct.
func Conflicts(actual Record, desired model.Listener) bool {
	actual.Address = actual.Address.Unmap()
	if actual.Network != desired.Network || actual.Port != desired.Port {
		return false
	}
	want, err := netip.ParseAddr(desired.Address)
	if err != nil || !actual.Address.IsValid() {
		return true
	}
	want = want.Unmap()
	if want.WithZone("").IsUnspecified() || actual.DualStack {
		return true
	}
	if want.Is4() != actual.Address.Is4() {
		return false
	}
	if actual.Address.WithZone("").IsUnspecified() {
		return true
	}
	if want.IsLinkLocalUnicast() && want.Zone() != "" && actual.Address.Zone() != "" && want.Zone() != actual.Address.Zone() {
		return false
	}
	return want.WithZone("") == actual.Address.WithZone("")
}

// Matches requires the requested bind scope; overlap alone is not readiness.
// An IPv4 wildcard can be served by an observed dual-stack Go listener.
func Matches(actual Record, desired model.Listener) bool {
	actual.Address = actual.Address.Unmap()
	if actual.Network != desired.Network || actual.Port != desired.Port {
		return false
	}
	want, err := netip.ParseAddr(desired.Address)
	if err != nil || !actual.Address.IsValid() {
		return false
	}
	want = want.Unmap()
	if want.Zone() != "" && want.Zone() != actual.Address.Zone() {
		return false
	}
	if want.WithZone("").IsUnspecified() {
		return actual.Address.WithZone("").IsUnspecified() && actual.Interface == "" && (want.Is4() == actual.Address.Is4() || want.Is4() && actual.DualStack)
	}
	return want.WithZone("") == actual.Address.WithZone("")
}
