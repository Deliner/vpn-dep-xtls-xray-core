package acceptance

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"vpn-script/internal/control/fleet/registry"
	"vpn-script/internal/platform/fleetapi"
)

func startController(t *testing.T, root, binary string) *controller {
	t.Helper()
	c := &controller{root: filepath.Join(root, "controller"), binary: binary}
	command(t, binary, "init", "--state", c.root, "--fleet", "acceptance")
	c.authority(t)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	require(t, err)
	c.address, c.endpoint = listener.Addr().String(), "https://"+listener.Addr().String()
	require(t, listener.Close())
	c.client, err = fleetapi.New(c.endpoint, c.ca)
	require(t, err)
	c.admin, err = registry.ReadAdminCredential(filepath.Join(c.root, "admin-credential.json"))
	require(t, err)
	t.Cleanup(func() { c.close(t) })
	c.reopen(t)
	return c
}

func (c *controller) reopen(t *testing.T) {
	t.Helper()
	if c.child != nil {
		t.Fatal("refusing to start a second controller process")
	}
	c.starts++
	var err error
	c.log, err = os.OpenFile(filepath.Join(filepath.Dir(c.root), "reports", fmt.Sprintf("controller-%d.log", c.starts)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	require(t, err)
	child := exec.Command(c.binary, "serve", "--state", c.root, "--cert", filepath.Join(c.root, "ca.pem"),
		"--key", filepath.Join(c.root, "key.pem"), "--listen", c.address)
	child.Stdout, child.Stderr = c.log, c.log
	if err := child.Start(); err != nil {
		c.log.Close()
		t.Fatal(err)
	}
	c.child, c.done = child, make(chan error, 1)
	done := c.done
	go func() { done <- child.Wait() }()
	eventually(t, "production controller HTTPS API", 15*time.Second, func() bool {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		_, err := c.client.List(ctx, c.admin)
		return err == nil
	})
}

func (c *controller) close(t *testing.T) {
	t.Helper()
	c.client.Close()
	if c.child == nil {
		return
	}
	err := c.child.Process.Signal(syscall.SIGTERM)
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		t.Error(err)
	}
	require(t, c.wait(t))
}

func (c *controller) wait(t *testing.T) error {
	t.Helper()
	defer func() {
		require(t, c.log.Close())
		c.child, c.done, c.log = nil, nil, nil
	}()
	select {
	case err := <-c.done:
		return err
	case <-time.After(15 * time.Second):
		c.child.Process.Kill()
		<-c.done
		t.Fatal("controller process did not terminate within deadline")
		return nil
	}
}

func (c *controller) kill(t *testing.T) *controllerCrashReport {
	t.Helper()
	proof := &controllerCrashReport{PID: c.child.Process.Pid, Signal: "SIGKILL", SignalNumber: int(syscall.SIGKILL)}
	child := c.child
	require(t, child.Process.Kill())
	err := c.wait(t)
	var exit *exec.ExitError
	status, ok := child.ProcessState.Sys().(syscall.WaitStatus)
	if !errors.As(err, &exit) || !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatal("controller did not terminate by the requested SIGKILL", err)
	}
	proof.Signaled = true
	return proof
}
