package probe_test

import (
	"context"
	"net"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/control/probe"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
)

func TestSSHConfiguredPortAndInconclusiveProbe(t *testing.T) {
	node := model.DeployNode{Address: "example.invalid", SSH: model.SSH{Port: 22022, User: "admin", ClosedPorts: []int{22}}}
	run := func(_ context.Context, _ time.Duration, args ...string) (process.Result, error) {
		if args[2] != "22022" {
			t.Fatal("SSH probe ignored configured port")
		}
		return process.Result{Success: true}, nil
	}
	cases := []struct {
		name string
		err  error
		pass bool
	}{
		{"refused", syscall.ECONNREFUSED, true}, {"dns-timeout", &net.DNSError{IsTimeout: true}, false}, {"network-error", syscall.ENETUNREACH, false},
	}
	for _, value := range cases {
		t.Run(value.name, func(t *testing.T) {
			dial := func(_ context.Context, _, address string) (net.Conn, error) {
				if address != "example.invalid:22" {
					t.Fatal("wrong obsolete port checked")
				}
				return nil, value.err
			}
			result := probe.SSH(context.Background(), node, run, dial)
			if result.Pass != value.pass {
				t.Fatalf("incorrect SSH verification result: %+v", result)
			}
		})
	}
}
