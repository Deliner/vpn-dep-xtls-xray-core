package validate_test

import (
	"testing"

	"vpn-script/internal/core/model"
	"vpn-script/internal/core/validate"
)

func TestSocketOverlapUsesTransportAndCanonicalAddresses(t *testing.T) {
	for _, check := range []struct {
		name, left, right, network string
		port                       int
		overlaps                   bool
	}{
		{"same-tcp", "127.0.0.1", "127.0.0.1", "tcp", 443, true},
		{"different-protocol", "127.0.0.1", "127.0.0.1", "udp", 443, false},
		{"different-port", "127.0.0.1", "127.0.0.1", "tcp", 8443, false},
		{"different-v4", "127.0.0.1", "127.0.0.2", "tcp", 443, false},
		{"mapped-v4", "127.0.0.1", "::ffff:127.0.0.1", "tcp", 443, true},
		{"different-mapped-v4", "127.0.0.1", "::ffff:127.0.0.2", "tcp", 443, false},
		{"wildcard-v4", "0.0.0.0", "127.0.0.1", "tcp", 443, true},
		{"mapped-wildcard", "::ffff:0.0.0.0", "127.0.0.1", "tcp", 443, true},
		{"same-v6", "::1", "0:0:0:0:0:0:0:1", "tcp", 443, true},
		{"different-v6", "::1", "::2", "tcp", 443, false},
		{"wildcard-v6", "::", "::1", "tcp", 443, true},
		{"dual-stack-wildcard", "::", "127.0.0.1", "tcp", 443, true},
		{"unknown-address", "invalid", "127.0.0.1", "tcp", 443, true},
		{"scoped-wildcard", "::%eth0", "::1", "tcp", 443, true},
		{"scoped-alias", "fe80::1%eth0", "fe80::1%eth1", "tcp", 443, true},
	} {
		t.Run(check.name, func(t *testing.T) {
			left := model.Listener{Tag: "first", Internal: true, Address: check.left, Network: "tcp", Port: 443}
			right := model.Listener{Tag: "second", Address: check.right, Network: check.network, Port: check.port}
			if validate.SocketsOverlap(left, right) != check.overlaps || validate.SocketsOverlap(right, left) != check.overlaps {
				t.Fatalf("incorrect or asymmetric overlap for %+v and %+v", left, right)
			}
		})
	}
}
