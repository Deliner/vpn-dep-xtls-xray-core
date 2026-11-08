package connectip

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/apernet/quic-go"
	"github.com/apernet/quic-go/http3"
	"github.com/stretchr/testify/require"
)

type budgetStream struct {
	packets     int
	bytes       int
	err         error
	readCancel  *quic.StreamErrorCode
	writeCancel *quic.StreamErrorCode
}

func (s *budgetStream) SetDatagramReceiveBuffer(packets, bytes int) error {
	s.packets, s.bytes = packets, bytes
	return s.err
}

func (s *budgetStream) CancelRead(code quic.StreamErrorCode) {
	s.readCancel = &code
}

func (s *budgetStream) CancelWrite(code quic.StreamErrorCode) {
	s.writeCancel = &code
}

func TestCONNECTIPReceiveBudgetIsBounded(t *testing.T) {
	stream := &budgetStream{}
	require.NoError(t, prepareDatagramReceive(stream))
	require.Equal(t, 1024, stream.packets)
	require.Equal(t, 1<<20, stream.bytes)
	require.Nil(t, stream.readCancel)
	require.Nil(t, stream.writeCancel)
}

func TestCONNECTIPReceiveBudgetFailureClosesBothDirections(t *testing.T) {
	failure := errors.New("already queued payload exceeds the budget")
	stream := &budgetStream{err: failure}
	require.ErrorIs(t, prepareDatagramReceive(stream), failure)
	code := quic.StreamErrorCode(http3.ErrCodeRequestCanceled)
	require.Equal(t, &code, stream.readCancel)
	require.Equal(t, &code, stream.writeCancel)
}

// Exercise the actual client and proxy setup, rather than configuring the
// streams from the test. DATAGRAM delivery is unordered; IDs must be unique.
func TestCONNECTIPDelayedReaderBurst(t *testing.T) {
	client, server := setupConns(t)
	t.Run("client to server", func(t *testing.T) {
		checkDelayedDatagrams(t, client.h3, server.h3)
	})
	t.Run("server to client", func(t *testing.T) {
		checkDelayedDatagrams(t, server.h3, client.h3)
	})
}

func checkDelayedDatagrams(t *testing.T, sender, receiver http3Stream) {
	t.Helper()
	const packets = 256
	for i := range packets {
		payload := bytes.Repeat([]byte{0x53}, 900)
		binary.BigEndian.PutUint32(payload[:4], uint32(i))
		require.NoError(t, sender.SendDatagram(payload))
		// Keep lower transport queues draining while the HTTP/3 consumer waits.
		time.Sleep(time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	seen := make(map[uint32]bool, packets)
	for range packets {
		payload, err := receiver.ReceiveDatagram(ctx)
		require.NoError(t, err, "received %d/%d delayed datagrams", len(seen), packets)
		require.Len(t, payload, 900)
		id := binary.BigEndian.Uint32(payload[:4])
		require.Less(t, id, uint32(packets))
		require.False(t, seen[id], "duplicate datagram %d", id)
		require.Equal(t, bytes.Repeat([]byte{0x53}, 896), payload[4:])
		seen[id] = true
	}
}
