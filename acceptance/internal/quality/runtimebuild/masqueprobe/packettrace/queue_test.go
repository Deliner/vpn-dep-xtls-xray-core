package packettrace

import (
	"bytes"
	"strings"
	"testing"
)

func TestNHOPQueueSnapshotOwnsOnlyImmutableMetadata(t *testing.T) {
	for _, packet := range [][]byte{nhopTestPacket(4), nhopTestPacket(6), nhopTestDatagram(4), nhopTestDatagram(6), nil} {
		original := bytes.Clone(packet)
		line := nhopServerQueueSnapshot(&packet, &packet, packet)
		if !bytes.Equal(packet, original) {
			t.Fatal("queue snapshot changed its producer's buffer")
		}
		for _, field := range []string{"pid=", "tunnel=0x", "packet=0x", "bytes=", "network="} {
			if !strings.Contains(line, field) {
				t.Fatalf("missing queue observation %q: %s", field, line)
			}
		}
		if strings.Contains(line, "private") || strings.Contains(line, "70726976617465") || strings.ContainsAny(line, "\r\n") {
			t.Fatal("queue snapshot disclosed payload or lost framing")
		}
		clear(packet)
		nhopTraceServerQueue("offer", line, 7, 512, false)
	}
}
