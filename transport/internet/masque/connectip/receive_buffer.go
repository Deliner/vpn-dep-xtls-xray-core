package connectip

import (
	"fmt"

	"github.com/apernet/quic-go"
	"github.com/apernet/quic-go/http3"
)

const (
	maxBufferedIPDatagrams = 1024
	maxBufferedIPBytes     = 1 << 20
)

type datagramReceiveBudget interface {
	SetDatagramReceiveBuffer(packets, bytes int) error
	CancelRead(quic.StreamErrorCode)
	CancelWrite(quic.StreamErrorCode)
}

// ConfigureQUICReceive bounds the shared connection queue before any incoming
// packet can be handled. It matches one CONNECT-IP stream's finite burst budget;
// multiple streams share this connection-wide cap rather than multiplying it.
func ConfigureQUICReceive(config *quic.Config) error {
	if err := config.SetDatagramReceiveBuffer(maxBufferedIPDatagrams, maxBufferedIPBytes); err != nil {
		return fmt.Errorf("connect-ip: failed to set QUIC receive budget: %w", err)
	}
	return nil
}

// Each authenticated CONNECT-IP stream owns one finite burst budget. Other
// HTTP/3 streams retain their default queue, and HTTP/2 capsule bounds are
// unchanged. Configure before address assignment / exchanging IP packets.
func prepareDatagramReceive(stream datagramReceiveBudget) error {
	if err := stream.SetDatagramReceiveBuffer(maxBufferedIPDatagrams, maxBufferedIPBytes); err != nil {
		stream.CancelRead(quic.StreamErrorCode(http3.ErrCodeRequestCanceled))
		stream.CancelWrite(quic.StreamErrorCode(http3.ErrCodeRequestCanceled))
		return fmt.Errorf("connect-ip: failed to set bounded datagram receive buffer: %w", err)
	}
	return nil
}
