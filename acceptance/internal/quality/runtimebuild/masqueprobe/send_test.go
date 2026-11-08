package masqueprobe

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sendFixture = `package quic
import (
	"errors"
)
func (h *sendQueue) Run() error {
	for {
		switch {
		case true:
			if err := h.conn.Write(e.buf.Data, e.gsoSize, e.ecn); err != nil {
				var tooLarge *DatagramTooLargeError
				if !isSendMsgSizeErr(err) && !errors.As(err, &tooLarge) {
					return err
				}
			}
			e.buf.Release()
		}
	}
}
`

func TestSendProbePreservesWriteErrorAndBufferOwnership(t *testing.T) {
	modified, err := instrumentSendQueue([]byte(sendFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, invariant := range []string{
		"h.conn.Write(e.buf.Data, e.gsoSize, e.ecn)", "e.buf.Release()", "return err",
		"if !isSendMsgSizeErr(err) && !errors.As(err, &tooLarge)", "NHOP_QUIC_SEND_SIZE_DROP",
	} {
		if bytes.Count(modified, []byte(invariant)) != 1 {
			t.Fatalf("diagnostic changed ownership or error handling: %s", invariant)
		}
	}
	for _, input := range [][]byte{modified, []byte(strings.ReplaceAll(sendFixture, "return err", "return nil"))} {
		if _, err := instrumentSendQueue(input); err == nil {
			t.Fatal("accepted an ambiguous or already instrumented send path")
		}
	}
}

func TestSendProbeRejectsChangedSourceAndOccupiedControl(t *testing.T) {
	path := filepath.Join(t.TempDir(), "send_queue.go")
	if err := os.WriteFile(path, []byte(sendFixture), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := instrumentVerifiedSend(path); err == nil {
		t.Fatal("accepted an unreviewed send queue")
	}
	if _, err := prepareSendControl(path); err == nil {
		t.Fatal("overwrote an existing diagnostic control path")
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != sendFixture {
		t.Fatal("rejected diagnostic changed input source")
	}
}
