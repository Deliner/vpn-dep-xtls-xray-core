package masqueprobe

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const queueFixture = `package http3
import (
	"errors"
	"os"
)
const streamDatagramQueueLen = 32
func (s *stream) enqueueDatagram(data []byte) {
	if len(s.queue) >= streamDatagramQueueLen {
		return
	}
	s.queue = append(s.queue, data)
}
`

const datagramFixture = `package quic
import (
	"context"
)
const maxDatagramRcvQueueLen = 128
func (h *datagramQueue) HandleDatagramFrame(f *wire.DatagramFrame) {
	data := make([]byte, len(f.Data))
	copy(data, f.Data)
	var queued bool
	h.rcvMx.Lock()
	if len(h.rcvQueue) < maxDatagramRcvQueueLen {
		h.rcvQueue = append(h.rcvQueue, data)
		queued = true
	}
	h.rcvMx.Unlock()
	if !queued && h.logger.Debug() {
		h.logger.Debugf("discarded")
	}
}
func (h *datagramQueue) Receive(ctx context.Context) {
	h.rcvMx.Lock()
	h.rcvMx.Unlock()
}
`

func TestConnectionProbePreservesAdmissionAndBoundsObservation(t *testing.T) {
	source := []byte(datagramFixture)
	modified, err := instrumentDatagramQueue(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, invariant := range []string{"const maxDatagramRcvQueueLen = 128", "len(h.rcvQueue) < maxDatagramRcvQueueLen", "h.rcvQueue = append(h.rcvQueue, data)", "data[:min(96, len(data))]", "if !queued && h.logger.Debug()"} {
		if bytes.Count(modified, []byte(invariant)) != 1 {
			t.Fatalf("changed admission or missing bounded observation: %s", invariant)
		}
	}
	if string(source) != datagramFixture || bytes.Count(modified, []byte("nhopQueued :=")) != 1 {
		t.Fatal("probe mutated input or instrumented an unrelated receive lock")
	}
	if _, err := instrumentDatagramQueue(modified); err == nil {
		t.Fatal("accepted duplicate instrumentation")
	}
}

func TestConnectionProbeRejectsChangedSourceAndExistingControl(t *testing.T) {
	path := filepath.Join(t.TempDir(), "datagram_queue.go")
	if err := os.WriteFile(path, []byte(datagramFixture), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := instrumentVerifiedReceive(path); err == nil {
		t.Fatal("accepted unreviewed connection queue source")
	}
	if _, err := prepareReceiveControl(path, datagramQueueSource); err == nil {
		t.Fatal("accepted existing control target")
	}
	if _, err := prepareReceiveControl(path+".absent", "unreviewed"); err == nil {
		t.Fatal("accepted unreviewed queue policy")
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != datagramFixture {
		t.Fatal("rejected observation changed source")
	}
}

func TestConfiguredConnectionProbePreservesBothBudgets(t *testing.T) {
	fixture := strings.Replace(datagramFixture, "len(h.rcvQueue) < maxDatagramRcvQueueLen", "h.rcvQueue.Len() < h.rcvMaxPackets && len(f.Data) <= h.rcvMaxBytes-h.rcvBytes", 1)
	fixture = strings.Replace(fixture, "h.rcvQueue = append(h.rcvQueue, data)", "h.rcvQueue.PushBack(data)", 1)
	modified, err := instrumentBudgetDatagramQueue([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, invariant := range []string{"h.rcvQueue.Len() < h.rcvMaxPackets", "len(f.Data) <= h.rcvMaxBytes-h.rcvBytes", "h.rcvQueue.PushBack(data)", "nhopBuffered := h.rcvBytes", "f.Data[:min(96, len(f.Data))]"} {
		if bytes.Count(modified, []byte(invariant)) != 1 {
			t.Fatalf("lost budget or observation: %s", invariant)
		}
	}
	if _, err := instrumentBudgetDatagramQueue(modified); err == nil {
		t.Fatal("accepted repeated budget instrumentation")
	}
}

func TestConnectionControlsMatchReviewedConstructorAndPolicy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "control.go")
	for _, digest := range []string{datagramQueueSource, datagramBudgetSource} {
		control, err := prepareReceiveControl(path, digest)
		if err != nil {
			t.Fatal(err)
		}
		configured := digest == datagramBudgetSource
		if bytes.Contains(control, []byte("utils.DefaultLogger, nil)")) != configured || bytes.Contains(control, []byte("TestNHOPReceiveQueueConfiguredBudget")) != configured {
			t.Fatal("control constructor or budget policy differs from source version")
		}
	}
}

func TestProbeRetainsQueueBoundAndNormalEnqueue(t *testing.T) {
	data := []byte(queueFixture)
	modified, err := instrumentMasqueQueue(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, invariant := range []string{"const streamDatagramQueueLen = 32", "s.queue = append(s.queue, data)", "NHOP_H3_QUEUE_DROP", "data[:min(64, len(data))]"} {
		if !bytes.Contains(modified, []byte(invariant)) {
			t.Fatalf("diagnostic changed a bound or lost observation: %s", invariant)
		}
	}
	if string(data) != queueFixture {
		t.Fatal("probe mutated its source buffer")
	}
	if _, err := instrumentMasqueQueue(modified); err == nil {
		t.Fatal("probe silently instrumented an already modified source")
	}
}

func TestBudgetProbeDoesNotChangeQueueAdmission(t *testing.T) {
	fixture := strings.Replace(queueFixture, "len(s.queue) >= streamDatagramQueueLen", "!s.queue.push(data)", 1)
	fixture = strings.Replace(fixture, "\ts.queue = append(s.queue, data)\n", "", 1)
	modified, err := instrumentMasqueQueue([]byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(modified, []byte("s.queue.push(data)")) != 1 || bytes.Contains(modified, []byte("append(s.queue")) {
		t.Fatal("probe changed or duplicated bounded queue admission")
	}
	if !bytes.Contains(modified, []byte("s.queue.count")) || !bytes.Contains(modified, []byte("NHOP_H3_QUEUE_DROP")) {
		t.Fatal("probe did not observe the configured queue")
	}
	if _, err := instrumentMasqueQueue(modified); err == nil {
		t.Fatal("probe instrumented a modified bounded queue again")
	}
}

func TestProbeRejectsUnreviewedSourceWithoutReplacingFiles(t *testing.T) {
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, "http3"), 0700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(directory, "http3", "state_tracking_stream.go")
	if err := os.WriteFile(source, []byte(queueFixture), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Overlay(directory, directory, directory); err == nil {
		t.Fatal("probe accepted a changed dependency source")
	}
	actual, err := os.ReadFile(source)
	if err != nil || string(actual) != queueFixture {
		t.Fatal("rejected probe altered the source")
	}
	if _, err := os.Stat(filepath.Join(directory, "overlay.json")); !os.IsNotExist(err) {
		t.Fatal("rejected probe published an overlay")
	}
}
