package acceptance

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"vpn-script/internal/platform/process"
)

func udpOrigin(t *testing.T) {
	t.Helper()
	listener, err := net.ListenPacket("udp", "198.18.0.2:18081")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	go func() {
		buffer := make([]byte, 1024)
		for {
			n, address, err := listener.ReadFrom(buffer)
			if err != nil {
				return
			}
			_, _ = listener.WriteTo(buffer[:n], address)
		}
	}()
}

func checkTraffic(t *testing.T, c *controller, allowed bool) {
	t.Helper()
	for _, operation := range []string{"probe", "probe6", "probe-udp"} {
		result := c.call(t, operation)
		if (result.Error == "") != allowed {
			t.Fatalf("%s allowed=%v: %s", operation, allowed, result.Error)
		}
	}
}

func directCanary(t *testing.T) *atomic.Uint64 {
	t.Helper()
	listener, err := net.ListenPacket("udp4", "198.51.100.7:18082")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	var received atomic.Uint64
	go func() {
		var data [128]byte
		for {
			n, peer, err := listener.ReadFrom(data[:])
			if err != nil {
				return
			}
			received.Add(1)
			listener.WriteTo(data[:n], peer)
		}
	}()
	return &received
}

type leakWatch struct {
	command  *exec.Cmd
	input    io.WriteCloser
	output   *json.Decoder
	cancel   context.CancelFunc
	finished bool
	attempts uint64
}

func startLeakWatch(t *testing.T) *leakWatch {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	command := process.Command(ctx, "ip", "netns", "exec", "nhop-client-inner", binary, "-test.run=^TestDirectLeakSender$")
	command.Env = append(os.Environ(), "NHOP_DIRECT_LEAK_WATCH=1")
	command.Stderr = os.Stderr
	input, err := command.StdinPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err == nil {
		err = command.Start()
	}
	if err != nil {
		cancel()
		t.Fatal("cannot start isolated leak monitor", err)
	}
	watch := &leakWatch{command: command, input: input, output: json.NewDecoder(output), cancel: cancel}
	t.Cleanup(func() { watch.stop(t) })
	var ready bool
	if err := watch.output.Decode(&ready); err != nil || !ready {
		t.Fatal("leak monitor did not become ready", err)
	}
	return watch
}

func (w *leakWatch) stop(t *testing.T) {
	t.Helper()
	if w.finished {
		return
	}
	w.finished = true
	defer w.cancel()
	w.input.Close()
	readErr := w.output.Decode(&w.attempts)
	waitErr := w.command.Wait()
	if readErr != nil || waitErr != nil {
		t.Error("leak monitor did not finish cleanly", readErr, waitErr)
	}
}

func TestDirectLeakSender(t *testing.T) {
	if os.Getenv("NHOP_DIRECT_LEAK_WATCH") != "1" {
		t.Skip("isolated direct leak sender only")
	}
	checkIsolation(t)
	connection, err := net.Dial("udp4", "198.51.100.7:18082")
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	done := make(chan struct{})
	go func() { io.Copy(io.Discard, os.Stdin); close(done) }()
	output := json.NewEncoder(os.Stdout)
	if err := output.Encode(true); err != nil {
		t.Fatal(err)
	}
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	var attempted uint64
	for {
		select {
		case <-done:
			if err := output.Encode(attempted); err != nil {
				t.Fatal(err)
			}
			return
		case <-tick.C:
			connection.Write([]byte("nhop-reconnect-direct-canary"))
			attempted++
		}
	}
}
