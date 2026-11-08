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
	if err := Overlay(directory, directory); err == nil {
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
