package probe

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"vpn-script/internal/control/client"
	"vpn-script/internal/control/probe/ipcheck"
	"vpn-script/internal/platform/process"
)

func TestProbeObservesRuntimeExitBeforeReadiness(t *testing.T) {
	for _, code := range []int{0, 23} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			binary := filepath.Join(t.TempDir(), "owned-runtime")
			if err := os.WriteFile(binary, []byte(fmt.Sprintf("#!/bin/sh\nexit %d\n", code)), 0700); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			started := time.Now()
			local, err := localClient(ctx, binary, t.TempDir(), probeRuntimeConfig("xray"))
			t.Logf("exit=%d observed in %s: %v", code, time.Since(started), err)
			if local != nil {
				local.stop()
			}
			if err == nil || errors.Is(err, context.DeadlineExceeded) || local != nil {
				t.Fatalf("runtime exit was not observed: client=%v err=%v", local, err)
			}
			var exit *exec.ExitError
			if code != 0 && (!errors.As(err, &exit) || exit.ExitCode() != code) {
				t.Fatalf("runtime exit status was lost: %v", err)
			}
		})
	}
}

func TestPinnedRuntimeIPQuery(t *testing.T) {
	for _, engine := range []string{"xray", "sing-box"} {
		t.Run(engine, func(t *testing.T) {
			variable := "XRAY_BINARY"
			if engine == "sing-box" {
				variable = "SING_BOX_BINARY"
			}
			binary := os.Getenv(variable)
			if binary == "" {
				t.Skip("set " + variable + " to run the real local SOCKS client")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := client.ValidateBinary(ctx, engine, binary, process.Run); err != nil {
				t.Fatal(err)
			}
			local, err := localClient(ctx, binary, t.TempDir(), probeRuntimeConfig(engine))
			if err != nil {
				t.Fatal(err)
			}
			defer local.stop()
			httpClient, endpoint := runtimeHTTPS(t, local.port)
			actual, err := ipcheck.Query(local.ctx, httpClient, endpoint)
			if err != nil || actual != "198.51.100.42" {
				t.Fatalf("real %s SOCKS probe failed: %q %v", engine, actual, err)
			}
		})
	}
}

func TestExitIPComparisonUsesAddressIdentity(t *testing.T) {
	for _, test := range []struct {
		expected, actual string
		pass             bool
	}{
		{"198.51.100.42", "::ffff:198.51.100.42", true},
		{"2001:0db8::42", "2001:db8::42", true},
		{"198.51.100.42", "198.51.100.43", false},
		{"", "198.51.100.42", false},
		{"fe80::1%eth0", "fe80::1%eth0", false},
	} {
		if actual := sameExitIP(test.expected, test.actual); actual != test.pass {
			t.Errorf("comparison %q/%q: got %v", test.expected, test.actual, actual)
		}
	}
}
