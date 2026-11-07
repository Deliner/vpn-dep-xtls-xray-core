package deploy

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/control/bundle"
	"vpn-script/internal/control/probe"
	"vpn-script/internal/control/topology"
	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/process"
	"vpn-script/internal/platform/safeio"
)

func workflowFixture(t testing.TB) (*model.Topology, string, string) {
	t.Helper()
	top, err := topology.Load("../../../examples/nhop/topology.yaml")
	if err != nil {
		t.Fatal(err)
	}
	output, installers := filepath.Join(t.TempDir(), "bundle"), workflowInstallers(t)
	if err := bundle.Render(top, output, installers); err != nil {
		t.Fatal(err)
	}
	return top, output, installers
}

func workflowInstallers(t testing.TB) string {
	t.Helper()
	installers := t.TempDir()
	for arch, machine := range map[string]uint16{"amd64": 62, "arm64": 183} {
		raw := make([]byte, 64)
		copy(raw, []byte{'\x7f', 'E', 'L', 'F', 2, 1, 1})
		binary.LittleEndian.PutUint16(raw[16:], 2)
		binary.LittleEndian.PutUint16(raw[18:], machine)
		binary.LittleEndian.PutUint32(raw[20:], 1)
		binary.LittleEndian.PutUint16(raw[52:], 64)
		if err := os.WriteFile(filepath.Join(installers, "nhop-node-linux-"+arch), raw, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return installers
}

func TestDeployAndVerifyRejectBusyBundleBeforeExternalWork(t *testing.T) {
	_, directory, _ := workflowFixture(t)
	lock, err := safeio.Lock(filepath.Join(directory, ".render.lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	run := func(context.Context, time.Duration, ...string) (process.Result, error) {
		t.Error("external work started with a busy bundle")
		return process.Result{}, fmt.Errorf("unexpected external work")
	}
	if err := Run(context.Background(), directory, run, io.Discard); !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Errorf("deploy must reject the active render lock: %v", err)
	}
	_, err = probe.Verify(context.Background(), directory, nil, "https://example.invalid", run)
	if !errors.Is(err, syscall.EWOULDBLOCK) {
		t.Errorf("verify must reject the active render lock: %v", err)
	}
}

func TestVerifyHoldsBundleUntilPreflightReturns(t *testing.T) {
	top, directory, installers := workflowFixture(t)
	path := filepath.Join(directory, "nodes", "entry.json")
	before, err := safeio.FileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	run := func(context.Context, time.Duration, ...string) (process.Result, error) {
		called = true
		endpoint := top.Services["entry.reality"]
		endpoint.Service.Port++
		top.Services["entry.reality"] = endpoint
		if err := bundle.Render(top, directory, installers); !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Errorf("bundle changed during verification: %v", err)
		}
		return process.Result{}, fmt.Errorf("injected runtime validation failure")
	}
	_, err = probe.Verify(context.Background(), directory, map[string]string{"xray": "fixture-runtime"}, "https://example.invalid", run)
	if !called || err == nil {
		t.Fatalf("preflight was not exercised: called=%v err=%v", called, err)
	}
	after, err := safeio.FileHash(path)
	if err != nil || before != after {
		t.Fatalf("verified generation was replaced: %v", err)
	}
	if err := bundle.Render(top, directory, installers); err != nil {
		t.Fatalf("failed verification retained its lock: %v", err)
	}
}

func TestFailedDeployReleasesBundleForRender(t *testing.T) {
	top, directory, installers := workflowFixture(t)
	run := func(context.Context, time.Duration, ...string) (process.Result, error) {
		return process.Result{}, fmt.Errorf("injected remote failure")
	}
	if err := Run(context.Background(), directory, run, io.Discard); err == nil {
		t.Fatal("deployment unexpectedly succeeded")
	}
	if err := bundle.Render(top, directory, installers); err != nil {
		t.Fatalf("failed deployment retained its lock: %v", err)
	}
}
