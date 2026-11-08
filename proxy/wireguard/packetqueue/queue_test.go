package packetqueue

import (
	"errors"
	"os"
	"testing"
	"testing/synctest"

	"github.com/xtls/xray-core/common/buf"
)

func testBuffer() *buf.Buffer {
	packet := buf.New()
	packet.Write([]byte{1, 2, 3, 4})
	return packet
}

func TestBackpressureTransfersOwnership(t *testing.T) {
	synctest.Test(t, checkBackpressure)
}

func checkBackpressure(t *testing.T) {
	queue := New(1)
	first, second := testBuffer(), testBuffer()
	if err := queue.Push(first); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- queue.Push(second) }()
	synctest.Wait()
	if queue.Len() != 1 {
		t.Fatal("capacity exceeded")
	}
	select {
	case err := <-result:
		t.Fatalf("full queue admitted a writer: %v", err)
	default:
	}
	packet, err := queue.Pop()
	if err != nil || packet != first {
		t.Fatalf("wrong first packet: %v", err)
	}
	synctest.Wait()
	if err := <-result; err != nil || queue.Len() != 1 {
		t.Fatalf("waiting writer did not resume: %v", err)
	}
	queue.Close()
	if second.Len() != 0 || first.Len() != 4 || queue.Len() != 0 {
		t.Fatal("close did not release exactly the queued ownership")
	}
	first.Release()
}

func TestCloseUnblocksReadersAndWriters(t *testing.T) {
	synctest.Test(t, checkCloseWakeups)
}

func checkCloseWakeups(t *testing.T) {
	empty, full := New(1), New(1)
	held, pending := testBuffer(), testBuffer()
	if err := full.Push(held); err != nil {
		t.Fatal(err)
	}
	read, write := make(chan error, 1), make(chan error, 1)
	go func() { _, err := empty.Pop(); read <- err }()
	go func() { write <- full.Push(pending) }()
	synctest.Wait()
	empty.Close()
	full.Close()
	synctest.Wait()
	if !errors.Is(<-read, os.ErrClosed) || !errors.Is(<-write, os.ErrClosed) {
		t.Fatal("close did not unblock both directions")
	}
	if held.Len() != 0 || pending.Len() != 4 || full.Len() != 0 {
		t.Fatal("close released unadmitted ownership or retained queued buffers")
	}
	if err := full.Push(pending); !errors.Is(err, os.ErrClosed) || pending.Len() != 4 {
		t.Fatal("closed queue accepted or consumed caller ownership")
	}
	pending.Release()
	full.Close()
	empty.Close()
}
