package masqueprobe

import (
	"fmt"
	"os"
)

func prepareSendControl(path, sourceSHA string) ([]byte, error) {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return nil, fmt.Errorf("diagnostic control target must be absent: %s", path)
	}
	switch sourceSHA {
	case sendQueueSource:
		return []byte(sendControl), nil
	case sendQueueMTUSource:
		return []byte(mtuSendControl), nil
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
