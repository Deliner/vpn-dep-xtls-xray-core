package sockets_test

import (
	"reflect"
	"strings"
	"testing"

	"vpn-script/internal/node/sockets"
)

func TestParseKernelBindingsAndOwners(t *testing.T) {
	for _, check := range []struct {
		endpoint, details, address, device string
		dual                               bool
	}{
		{"127.0.0.1:443", "", "127.0.0.1", "", false},
		{"127.0.0.53%lo:443", "", "127.0.0.53", "lo", false},
		{"[::1]:443", "v6only:1", "::1", "", false},
		{"[::ffff:127.0.0.1]:443", "v6only:0", "127.0.0.1", "", false},
		{"[fe80::1%eth0]:443", "v6only:1", "fe80::1%eth0", "eth0", false},
		{"[::]:443", "v6only:1", "::", "", false},
		{"*:443", "v6only:0", "::", "", true},
		{"[::%eth0]:443", "v6only:0", "::%eth0", "eth0", true},
	} {
		for _, protocol := range []string{"tcp LISTEN", "udp UNCONN"} {
			raw := protocol + " 0 128 " + check.endpoint + " *:* users:((\"app\",pid=123,fd=7),(\"app\",pid=123,fd=8),(\"other\",pid=456,fd=9)) " + check.details + "\n"
			records, err := sockets.Parse(raw)
			if err != nil || len(records) != 1 {
				t.Fatalf("kernel binding %s: %v", check.endpoint, err)
			}
			actual := records[0]
			if actual.Address.String() != check.address || actual.Interface != check.device || actual.DualStack != check.dual || actual.Port != 443 || actual.Network != strings.Fields(protocol)[0] || !reflect.DeepEqual(actual.PIDs, []string{"123", "456"}) {
				t.Fatalf("incorrect socket observation: %+v", actual)
			}
		}
	}
}

func TestParseRejectsIncompleteAndAmbiguousObservations(t *testing.T) {
	for _, line := range []string{"tcp", "error reading sockets", "unix LISTEN 0 128 /path *:*",
		"tcp ESTAB 0 128 127.0.0.1:443 *:*", "udp ESTAB 0 128 127.0.0.1:443 *:*",
		"tcp LISTEN 0 128 invalid *:*", "tcp LISTEN 0 128 localhost:443 *:*",
		"tcp LISTEN 0 128 127.0.0.1:0 *:*", "tcp LISTEN 0 128 127.0.0.1:65536 *:*",
		"tcp LISTEN 0 128 [::]:443 *:*", "tcp LISTEN 0 128 *:443 *:*",
		"tcp LISTEN 0 128 *:443 *:* v6only:1", "tcp LISTEN 0 128 [::]:443 *:* v6only:unknown",
		"tcp LISTEN 0 128 [::]:443 *:* v6only:0 v6only:1",
	} {
		if records, err := sockets.Parse("tcp LISTEN 0 128 127.0.0.1:22 *:*\n" + line); err == nil || records != nil {
			t.Fatalf("partial socket inventory accepted: %q, %v", line, err)
		}
	}
}

func TestParseAllowsEmptyObservationAndUnattributedListeners(t *testing.T) {
	if records, err := sockets.Parse("\n\t\n"); err != nil || len(records) != 0 {
		t.Fatal("empty output was not an empty inventory", err)
	}
	records, err := sockets.Parse("tcp LISTEN 0 128 0.0.0.0:22 *:* uid:0 ino:12\n")
	if err != nil || len(records) != 1 || len(records[0].PIDs) != 0 {
		t.Fatal("unknown ownership must remain unknown", records, err)
	}
}
