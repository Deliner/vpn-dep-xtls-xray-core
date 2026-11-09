package connectip

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

type boundedDatagramSource struct {
	packets chan []byte
	calls   atomic.Int32
}

func (s *boundedDatagramSource) ReceiveDatagram(ctx context.Context) ([]byte, error) {
	s.calls.Add(1)
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case packet, ok := <-s.packets:
		if !ok {
			return nil, io.EOF
		}
		return packet, nil
	}
}

func TestNHOPDatagramReceiverBackpressure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &boundedDatagramSource{packets: make(chan []byte, 2)}
		source.packets <- []byte("first")
		source.packets <- []byte("second")
		receiver := newDatagramReceiver(source)
		defer receiver.stop()
		receiver.start()
		synctest.Wait()
		if source.calls.Load() != 1 {
			t.Fatal("receiver consumed more than one packet without a consumer")
		}
		for _, expected := range []string{"first", "second"} {
			if actual := <-receiver.packets; !bytes.Equal(actual, []byte(expected)) {
				t.Fatalf("datagram order or bytes changed: %q", actual)
			}
		}
		close(source.packets)
		<-receiver.done
		if !errors.Is(receiver.err, io.EOF) {
			t.Fatalf("source error lost: %v", receiver.err)
		}
	})
}

func TestNHOPDatagramReceiverStopsWhileBlocked(t *testing.T) {
	for _, waitingForConsumer := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			source := &boundedDatagramSource{packets: make(chan []byte, 1)}
			if waitingForConsumer {
				source.packets <- []byte("pending")
			}
			receiver := newDatagramReceiver(source)
			receiver.start()
			synctest.Wait()
			receiver.stop()
			receiver.stop()
			if !errors.Is(receiver.err, context.Canceled) {
				t.Fatalf("blocked receiver did not stop: %v", receiver.err)
			}
		})
	}
}

func TestNHOPDatagramReceiverConcurrentStop(t *testing.T) {
	for range 64 {
		synctest.Test(t, func(t *testing.T) {
			source := &boundedDatagramSource{packets: make(chan []byte, 1)}
			source.packets <- []byte("racing")
			receiver := newDatagramReceiver(source)
			go receiver.start()
			finished := make(chan struct{}, 2)
			for range 2 {
				go func() {
					receiver.stop()
					finished <- struct{}{}
				}()
			}
			<-finished
			<-finished
			if !errors.Is(receiver.err, context.Canceled) {
				t.Fatalf("shutdown failed: %v", receiver.err)
			}
		})
	}
}

func TestNHOPDatagramReceiverStopBeforeFirstRead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		source := &boundedDatagramSource{packets: make(chan []byte, 1)}
		source.packets <- []byte("unread")
		receiver := newDatagramReceiver(source)
		receiver.stop()
		receiver.start()
		synctest.Wait()
		if source.calls.Load() != 0 || !errors.Is(receiver.err, context.Canceled) {
			t.Fatalf("unread datagram was consumed: calls=%d err=%v", source.calls.Load(), receiver.err)
		}
	})
}
