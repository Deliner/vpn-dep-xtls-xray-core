package masque

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"testing"
)

func parseQueueDrop(line string) (queueDrop, error) {
	var event queueDrop
	var pid, stream, queued, size int
	var prefix string
	n, err := fmt.Sscanf(line, "NHOP_H3_QUEUE_DROP unix_ns=%d pid=%d stream=%d queued=%d bytes=%d prefix=%s",
		&event.UnixNS, &pid, &stream, &queued, &size, &prefix)
	if err != nil || n != 6 || event.UnixNS <= 0 || pid <= 0 || queued < 0 || size <= 0 {
		return event, fmt.Errorf("invalid diagnostic queue marker: %q", line)
	}
	data, err := hex.DecodeString(prefix)
	if err != nil || len(data) != min(size, 64) || len(data) < 21 || data[0] != 0 || data[1]>>4 != 4 {
		return event, fmt.Errorf("invalid synthetic IPv4 queue prefix: %q", prefix)
	}
	event.Protocol = data[10]
	if event.Protocol == 17 {
		event.ID, event.HasID = udpIdentity(data[1:])
	}
	return event, nil
}

func udpIdentity(ip []byte) (packetID, bool) {
	var id packetID
	if len(ip) < 20 || ip[0]>>4 != 4 || ip[9] != 17 {
		return id, false
	}
	header := int(ip[0]&15) * 4
	if header < 20 || len(ip) < header+24 || binary.BigEndian.Uint16(ip[6:8])&0x3fff != 0 {
		return id, false
	}
	udp := ip[header:]
	source, target := binary.BigEndian.Uint16(udp[:2]), binary.BigEndian.Uint16(udp[2:4])
	if source != udpOriginPort && target != udpOriginPort || binary.BigEndian.Uint16(udp[4:6]) != 908 {
		return id, false
	}
	data := udp[8:]
	id.Nonce = binary.BigEndian.Uint64(data[:8])
	id.Worker = binary.BigEndian.Uint32(data[8:12])
	id.Sequence = binary.BigEndian.Uint32(data[12:16])
	return id, true
}

func TestQueueMarkerIdentifiesOnlyCompleteSyntheticUDPHeaders(t *testing.T) {
	data := make([]byte, 929)
	data[1], data[10] = 0x45, 17
	binary.BigEndian.PutUint16(data[23:25], udpOriginPort)
	binary.BigEndian.PutUint16(data[25:27], 908)
	copy(data[29:], packet(7, 3, 19, 900))
	for _, queued := range []int{0, 32} {
		line := fmt.Sprintf("NHOP_H3_QUEUE_DROP unix_ns=123 pid=5 stream=0 queued=%d bytes=929 prefix=%x", queued, data[:64])
		event, err := parseQueueDrop(line)
		if err != nil || !event.HasID || event.ID != (packetID{7, 3, 19}) {
			t.Fatalf("synthetic packet correlation failed: %+v / %v", event, err)
		}
	}
	for _, size := range []int{0, 19, 27, 43} {
		if _, ok := udpIdentity(data[1 : size+1]); ok {
			t.Fatal("incomplete packet received an identity")
		}
	}
	data[7] = 0x20 // IPv4 more-fragments flag, not a complete UDP datagram.
	if _, ok := udpIdentity(data[1:64]); ok {
		t.Fatal("fragment was attributed as a complete UDP packet")
	}
}

func TestQueueMarkerRejectsBrokenEvidence(t *testing.T) {
	for _, line := range []string{"", "NHOP_H3_QUEUE_DROP", "NHOP_H3_QUEUE_DROP unix_ns=1 pid=1 stream=0 queued=32 bytes=929 prefix=00"} {
		if _, err := parseQueueDrop(line); err == nil {
			t.Fatal("malformed diagnostic marker accepted")
		}
	}
}
