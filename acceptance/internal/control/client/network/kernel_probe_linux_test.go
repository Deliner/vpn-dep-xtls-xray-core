package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"testing"
	"time"
)

type kernelRequest struct{ Network, Address, Action string }
type kernelResponse struct{ Allowed bool }
type kernelProbe struct {
	command *exec.Cmd
	input   io.WriteCloser
	output  *json.Decoder
}

func TestKernelProbeProcess(t *testing.T) {
	uid, err := strconv.Atoi(os.Getenv("NHOP_CLIENT_PROBE_UID"))
	if err != nil || uid < 100 {
		t.Skip("private kernel probe subprocess only")
	}
	if err := syscall.Setgid(uid); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Setuid(uid); err != nil {
		t.Fatal(err)
	}
	decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
	var held net.Conn
	defer func() {
		if held != nil {
			held.Close()
		}
	}()
	for {
		var request kernelRequest
		if err := decoder.Decode(&request); err == io.EOF {
			return
		} else if err != nil {
			t.Fatal(err)
		}
		if err := encoder.Encode(kernelResponse{Allowed: probeRequest(request, &held)}); err != nil {
			t.Fatal(err)
		}
	}
}

func probeRequest(request kernelRequest, held *net.Conn) bool {
	if request.Action == "dns" {
		return probeDNS(request.Address)
	}
	connection := *held
	if request.Action != "reuse" {
		var err error
		connection, err = net.DialTimeout(request.Network, request.Address, 500*time.Millisecond)
		if err != nil {
			return false
		}
	}
	if request.Action == "hold" {
		*held = connection
	} else if request.Action != "reuse" {
		defer connection.Close()
	}
	if connection == nil || connection.SetDeadline(time.Now().Add(500*time.Millisecond)) != nil {
		return false
	}
	if _, err := connection.Write([]byte{42}); err != nil {
		return false
	}
	var reply [1]byte
	_, err := io.ReadFull(connection, reply[:])
	return err == nil && reply[0] == 42
}

func startKernelProbe(t *testing.T, uid int) *kernelProbe {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("ip", "netns", "exec", "nhop-client-inner", binary, "-test.run=^TestKernelProbeProcess$")
	command.Env = append(os.Environ(), fmt.Sprintf("NHOP_CLIENT_PROBE_UID=%d", uid))
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		input.Close()
		if err := command.Wait(); err != nil {
			t.Errorf("kernel probe cleanup: %v", err)
		}
	})
	return &kernelProbe{command: command, input: input, output: json.NewDecoder(output)}
}

func (probe *kernelProbe) check(t *testing.T, request kernelRequest, allowed bool) {
	t.Helper()
	if err := json.NewEncoder(probe.input).Encode(request); err != nil {
		t.Fatal(err)
	}
	var result kernelResponse
	if err := probe.output.Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Allowed != allowed {
		t.Fatalf("%+v allowed=%v, expected=%v", request, result.Allowed, allowed)
	}
}
