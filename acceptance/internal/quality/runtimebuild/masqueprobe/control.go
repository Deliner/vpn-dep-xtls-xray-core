package masqueprobe

import (
	"fmt"
	"os"
)

func prepareSendControl(path string) ([]byte, error) {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		return nil, fmt.Errorf("diagnostic control target must be absent: %s", path)
	}
	return []byte(sendControl), nil
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
