package masqueprobe

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-script/internal/quality/runtimebuild/masqueprobe/overlay"
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
	if _, _, err := instrumentVerifiedSend(path); err == nil {
		t.Fatal("accepted an unreviewed send queue")
	}
	if _, err := prepareSendControl(path, sendQueueSource); err == nil {
		t.Fatal("overwrote an existing diagnostic control path")
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != sendFixture {
		t.Fatal("rejected diagnostic changed input source")
	}
}

const mtuSendFixture = `package quic
import (
	"errors"
	"fmt"
)
func (h *sendQueue) write(entry queueEntry) error {
	defer entry.buf.Release()
	err := h.conn.Write(entry.buf.Data, entry.gsoSize, entry.ecn)
	var tooLarge *DatagramTooLargeError
	if err == nil || !isSendMsgSizeErr(err) && !errors.As(err, &tooLarge) {
		return err
	}
	if entry.mtuProbe {
		return nil
	}
	return fmt.Errorf("non-probe packet: %w", err)
}
`

func TestMTUProbeInstrumentationPreservesFatalDataErrors(t *testing.T) {
	modified, err := instrumentMTUSendQueue([]byte(mtuSendFixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, invariant := range []string{"defer entry.buf.Release()", "h.conn.Write(entry.buf.Data, entry.gsoSize, entry.ecn)",
		"if entry.mtuProbe {", "return fmt.Errorf(\"non-probe packet: %w\", err)", "NHOP_QUIC_SEND_SIZE_DROP"} {
		if bytes.Count(modified, []byte(invariant)) != 1 {
			t.Fatalf("diagnostic changed data/probe policy: %s", invariant)
		}
	}
	for _, input := range [][]byte{modified, []byte(strings.ReplaceAll(mtuSendFixture, "entry.mtuProbe", "true"))} {
		if _, err := instrumentMTUSendQueue(input); err == nil {
			t.Fatal("accepted ambiguous MTU error policy")
		}
	}
	for _, sha := range []string{sendQueueSource, sendQueueMTUSource, "unknown"} {
		control, err := prepareSendControl(filepath.Join(t.TempDir(), "control.go"), sha)
		if (err == nil) != (sha != "unknown") || sha == sendQueueMTUSource && !bytes.Contains(control, []byte("SendMTUProbe")) {
			t.Fatalf("control does not match its reviewed source: %s %v", sha, err)
		}
	}
}

func TestAdaptiveProbePreservesRecoveryOwnership(t *testing.T) {
	recovery := "\tif h.reportMTUError(entry) {\n\t\treturn nil\n\t}\n"
	fixture := strings.Replace(mtuSendFixture, "\treturn fmt.Errorf", recovery+"\treturn fmt.Errorf", 1)
	modified, err := instrumentAdaptiveSendQueue([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, invariant := range []string{"defer entry.buf.Release()", "h.reportMTUError(entry)",
		"if entry.mtuProbe {", "return fmt.Errorf(\"non-probe packet: %w\", err)", "NHOP_QUIC_SEND_SIZE_DROP",
		"size = min(size, protocol.ByteCount(entry.gsoSize))", "size <= protocol.MinInitialPacketSize || h.mtuErrors == nil"} {
		if bytes.Count(modified, []byte(invariant)) != 1 {
			t.Fatalf("diagnostic changed recovery or minimum size policy: %s", invariant)
		}
	}
	for _, input := range [][]byte{modified, []byte(mtuSendFixture), []byte(strings.ReplaceAll(fixture, "entry.mtuProbe", "true"))} {
		if _, err := instrumentAdaptiveSendQueue(input); err == nil {
			t.Fatal("accepted ambiguous adaptive MTU policy")
		}
	}
}

func TestAdaptiveControlUsesAnExplicitRecoveryOwner(t *testing.T) {
	control, err := prepareSendControl(filepath.Join(t.TempDir(), "control.go"), sendQueueAdaptiveSource)
	if err != nil {
		t.Fatal(err)
	}
	for _, invariant := range []string{"newSendQueue(connection, nil)", "newSendQueue(connection, feedback)",
		"TestNHOPSendQueuePMTUFeedback", "TestNHOPSendQueuePMTUMinimum", "NHOP_ADAPTIVE_SEND_POLICY"} {
		if bytes.Count(control, []byte(invariant)) != 1 {
			t.Fatalf("control lost explicit recovery ownership: %s", invariant)
		}
	}
	if sendQueuePolicy([]overlay.Change{{SourceSHA: sendQueueAdaptiveSource}}) != "adaptive-pmtu" {
		t.Fatal("diagnostic manifest lost adaptive send policy")
	}
}
