package masqueprobe

import (
	"fmt"
	"os"
	"strings"
)

func prepareReceiveControl(path, sourceSHA string) ([]byte, error) {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return nil, fmt.Errorf("diagnostic control target must be absent: %s", path)
	}
	switch sourceSHA {
	case datagramQueueSource:
		return []byte(receiveControl), nil
	case datagramBudgetSource:
		legacy := strings.Replace(receiveControl, "utils.DefaultLogger)", "utils.DefaultLogger, nil)", 1)
		return []byte(legacy + receiveBudgetControl), nil
	default:
		return nil, fmt.Errorf("unreviewed QUIC receive policy")
	}
}

const receiveBudgetControl = `
func TestNHOPReceiveQueueConfiguredBudget(t *testing.T) {
	config := &Config{}
	if err := config.SetDatagramReceiveBuffer(2, 6); err != nil {
		t.Fatal(err)
	}
	queue := newDatagramQueue(func() {}, utils.DefaultLogger, config)
	for _, data := range []string{"abc", "def", "g"} {
		queue.HandleDatagramFrame(&wire.DatagramFrame{Data: []byte(data)})
	}
	actual, err := queue.Receive(context.Background())
	if err != nil || string(actual) != "abc" {
		t.Fatalf("packet limit changed retained data: %x %v", actual, err)
	}
	queue.HandleDatagramFrame(&wire.DatagramFrame{Data: []byte("toolong")})
	actual, err = queue.Receive(context.Background())
	if err != nil || string(actual) != "def" {
		t.Fatalf("byte limit changed retained data: %x %v", actual, err)
	}
	queue.CloseWithError(io.EOF)
	if _, err := queue.Receive(context.Background()); err != io.EOF {
		t.Fatalf("budget accepted rejected data or lost close: %v", err)
	}
	fmt.Println("NHOP_QUIC_CONFIGURED_POLICY packets=2 bytes=6 drops=2 preserved=true")
}
`

// Fill the real connection queue, force bounded drops, then verify independent
// copies, FIFO, recovery and close. The marker check belongs to the build recipe.
const receiveControl = `package quic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/apernet/quic-go/internal/utils"
	"github.com/apernet/quic-go/internal/wire"
)

func TestNHOPReceiveQueueOverflow(t *testing.T) {
	queue := newDatagramQueue(func() {}, utils.DefaultLogger)
	for sequence := range maxDatagramRcvQueueLen {
		data := []byte{byte(sequence)}
		queue.HandleDatagramFrame(&wire.DatagramFrame{Data: data})
		data[0] = 0xff
	}
	queue.HandleDatagramFrame(&wire.DatagramFrame{Data: bytes.Repeat([]byte{0xab}, 128)})
	queue.HandleDatagramFrame(&wire.DatagramFrame{Data: []byte{0xff}})
	for sequence := range maxDatagramRcvQueueLen {
		actual, err := queue.Receive(context.Background())
		if err != nil || !bytes.Equal(actual, []byte{byte(sequence)}) {
			t.Fatalf("receive copy/FIFO at %d: %x %v", sequence, actual, err)
		}
	}
	queue.HandleDatagramFrame(&wire.DatagramFrame{Data: []byte("after-overflow")})
	actual, err := queue.Receive(context.Background())
	if err != nil || string(actual) != "after-overflow" {
		t.Fatalf("queue failed to recover after overflow: %x %v", actual, err)
	}
	queue.CloseWithError(io.EOF)
	if _, err := queue.Receive(context.Background()); err != io.EOF {
		t.Fatalf("queue did not preserve close: %v", err)
	}
	fmt.Println("NHOP_QUIC_RECEIVE_POLICY limit=128 drops=2 fifo=true copy=true recovery=true close=true")
}
`

func prepareSendControl(path, sourceSHA string) ([]byte, error) {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return nil, fmt.Errorf("diagnostic control target must be absent: %s", path)
	}
	switch sourceSHA {
	case sendQueueSource:
		return []byte(sendControl), nil
	case sendQueueMTUSource:
		return []byte(mtuSendControl), nil
	case sendQueueAdaptiveSource:
		legacy := strings.Replace(mtuSendControl, "newSendQueue(connection)", "newSendQueue(connection, nil)", 1)
		return []byte(legacy + adaptiveSendControl), nil
	default:
		return nil, fmt.Errorf("unreviewed send error policy")
	}
}

// This test is overlaid into the verified dependency only for the probe build.
// Both size failures must remain nonfatal and the next packet must still be sent.
const sendControl = `package quic

import (
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/apernet/quic-go/internal/protocol"
	"go.uber.org/mock/gomock"
)

func TestNHOPSendQueueSizeDrop(t *testing.T) {
	for _, failure := range []error{syscall.EMSGSIZE, &DatagramTooLargeError{MaxDatagramPayloadSize: 1200}} {
		synctest.Test(t, func(t *testing.T) {
			connection := NewMockSendConn(gomock.NewController(t))
			queue := newSendQueue(connection)
			gomock.InOrder(
				connection.EXPECT().Write(gomock.Any(), uint16(1300), protocol.ECNNon).Return(failure),
				connection.EXPECT().Write([]byte("after-drop"), uint16(10), protocol.ECNNon).Return(nil),
			)
			finished := make(chan error, 1)
			go func() { finished <- queue.Run() }()
			queue.Send(getPacketWithContents(make([]byte, 1300)), 1300, protocol.ECNNon)
			queue.Send(getPacketWithContents([]byte("after-drop")), 10, protocol.ECNNon)
			synctest.Wait()
			queue.Close()
			if err := <-finished; err != nil {
				t.Fatalf("size failure changed queue error handling: %v", err)
			}
		})
	}
}
`

// With a recovery owner, larger data packets report their size and the queue
// continues. A failure at QUIC's minimum size must still terminate the queue.
const adaptiveSendControl = `
func TestNHOPSendQueuePMTUFeedback(t *testing.T) {
	for _, failure := range []error{syscall.EMSGSIZE, &DatagramTooLargeError{MaxDatagramPayloadSize: 1200}} {
		synctest.Test(t, func(t *testing.T) { observeAdaptiveMTUPolicy(t, failure, 1300) })
	}
}

func TestNHOPSendQueuePMTUMinimum(t *testing.T) {
	synctest.Test(t, func(t *testing.T) { observeAdaptiveMTUPolicy(t, syscall.EMSGSIZE, 1200) })
}

func observeAdaptiveMTUPolicy(t *testing.T, failure error, size int) {
	t.Helper()
	connection := NewMockSendConn(gomock.NewController(t))
	feedback := make(chan protocol.ByteCount, 1)
	queue := newSendQueue(connection, feedback)
	first := connection.EXPECT().Write(gomock.Any(), uint16(0), protocol.ECNNon).Return(failure)
	packet := getPacketWithContents(make([]byte, size))
	queue.Send(packet, 0, protocol.ECNNon)
	if size > 1200 {
		connection.EXPECT().Write([]byte("after-drop"), uint16(10), protocol.ECNNon).Return(nil).After(first.Call)
		queue.Send(getPacketWithContents([]byte("after-drop")), 10, protocol.ECNNon)
	}
	finished := make(chan error, 1)
	go func() { finished <- queue.Run() }()
	synctest.Wait()
	queue.Close()
	err := <-finished
	terminal := size <= 1200
	if terminal && !errors.Is(err, failure) || !terminal && err != nil || packet.refCount != 0 {
		t.Fatalf("changed adaptive MTU error or buffer ownership: %v refs=%d", err, packet.refCount)
	}
	if terminal && len(feedback) != 0 || !terminal && len(feedback) != 1 {
		t.Fatalf("changed adaptive MTU notification: %d", len(feedback))
	}
	if !terminal && <-feedback != protocol.ByteCount(size) {
		t.Fatal("changed rejected logical packet size")
	}
	fmt.Printf("NHOP_ADAPTIVE_SEND_POLICY size=%d terminal=%t preserved=true\n", size, terminal)
}
`

// The new policy distinguishes deliberately oversized discovery packets from
// ordinary data. The diagnostic must preserve both outcomes.
const mtuSendControl = `package quic

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/apernet/quic-go/internal/protocol"
	"go.uber.org/mock/gomock"
)

func TestNHOPSendQueueSizeDrop(t *testing.T) {
	for _, failure := range []error{syscall.EMSGSIZE, &DatagramTooLargeError{MaxDatagramPayloadSize: 1200}} {
		for _, probe := range []bool{false, true} {
			synctest.Test(t, func(t *testing.T) { observeMTUPolicy(t, failure, probe) })
		}
	}
}

func observeMTUPolicy(t *testing.T, failure error, probe bool) {
	t.Helper()
	connection := NewMockSendConn(gomock.NewController(t))
	queue := newSendQueue(connection)
	first := connection.EXPECT().Write(gomock.Any(), uint16(0), protocol.ECNNon).Return(failure)
	packet := getPacketWithContents(make([]byte, 1300))
	if probe {
		connection.EXPECT().Write([]byte("after-drop"), uint16(10), protocol.ECNNon).Return(nil).After(first.Call)
		queue.SendMTUProbe(packet, protocol.ECNNon)
		queue.Send(getPacketWithContents([]byte("after-drop")), 10, protocol.ECNNon)
	} else {
		queue.Send(packet, 0, protocol.ECNNon)
	}
	finished := make(chan error, 1)
	go func() { finished <- queue.Run() }()
	synctest.Wait()
	queue.Close()
	err := <-finished
	if probe && err != nil || !probe && !errors.Is(err, failure) {
		t.Fatalf("instrumentation changed MTU policy: probe=%t error=%v", probe, err)
	}
	fmt.Printf("NHOP_SEND_POLICY probe=%t terminal=%t preserved=true\n", probe, !probe)
}
`
