package sockets_test

import (
	"net/netip"
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/node/sockets"
)

func TestConcreteBindingsSeparateConflictFromReadiness(t *testing.T) {
	for _, check := range []struct {
		actual, desired string
		dual            bool
		conflict, ready bool
	}{
		{"127.0.0.1", "127.0.0.1", false, true, true},
		{"127.0.0.2", "127.0.0.1", false, false, false},
		{"127.0.0.1", "::ffff:127.0.0.1", false, true, true},
		{"::ffff:127.0.0.1", "127.0.0.1", false, true, true},
		{"0.0.0.0", "127.0.0.1", false, true, false},
		{"127.0.0.1", "0.0.0.0", false, true, false},
		{"::1", "::1", false, true, true},
		{"::2", "::1", false, false, false},
		{"::1", "127.0.0.1", false, false, false},
		{"::", "127.0.0.1", false, false, false},
		{"::", "127.0.0.1", true, true, false},
		{"::", "0.0.0.0", false, true, false},
		{"::", "0.0.0.0", true, true, true},
		{"0.0.0.0", "::", false, true, false},
		{"::", "::", false, true, true},
		{"::", "::", true, true, true},
		{"fe80::1%eth0", "fe80::1%eth1", false, false, false},
		{"fe80::1%eth0", "fe80::1%eth0", false, true, true},
		{"2001:db8::1%eth0", "2001:db8::1%eth1", false, true, false},
	} {
		actual := sockets.Record{Network: "tcp", Port: 443, Address: netip.MustParseAddr(check.actual), DualStack: check.dual}
		want := model.Listener{Network: "tcp", Port: 443, Address: check.desired}
		if sockets.Conflicts(actual, want) != check.conflict || sockets.Matches(actual, want) != check.ready {
			t.Fatalf("incorrect comparison: %+v / %+v", actual, want)
		}
	}
}

func TestTransportPortAndBindScopeRemainRequired(t *testing.T) {
	actual := sockets.Record{Network: "tcp", Port: 443, Address: netip.MustParseAddr("127.0.0.1")}
	for _, want := range []model.Listener{{Network: "udp", Port: 443, Address: "127.0.0.1"}, {Network: "tcp", Port: 8443, Address: "127.0.0.1"}} {
		if sockets.Conflicts(actual, want) || sockets.Matches(actual, want) {
			t.Fatal("different transport or port was matched", want)
		}
	}
	unknown := model.Listener{Network: "tcp", Port: 443, Address: "invalid"}
	if !sockets.Conflicts(actual, unknown) || sockets.Matches(actual, unknown) {
		t.Fatal("unknown desired address did not fail closed")
	}
	actual.Address, actual.Interface = netip.IPv4Unspecified(), "eth0"
	if sockets.Matches(actual, model.Listener{Network: "tcp", Port: 443, Address: "0.0.0.0"}) {
		t.Fatal("interface-limited wildcard treated as all interfaces")
	}
}
